<?php

namespace App\Modules\Probes\Services;

use App\Modules\Incidents\Jobs\ProcessStateChangeAlertJob;
use App\Modules\Incidents\Models\Incident;
use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Targets\Models\MonitoringTarget;
use App\Modules\Targets\Services\CredentialEncryptionService;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Log;

/**
 * Orchestrates a single probe execution end-to-end:
 *   adapter -> MonitoringLog insert -> Redis state -> state-change/incident logic.
 * docs/gemini/dokumen_arsitektur_aplikasi_web.md:145-181
 */
class ProbeExecutionService
{
    public function __construct(
        private readonly ProbeAdapterFactory $factory,
        private readonly TargetStateService $state,
        private readonly CredentialEncryptionService $encryption,
        private readonly ProbeFailureLogger $failures,
    ) {
    }

    public function execute(MonitoringTarget $target): ProbeResult
    {
        $adapter = $this->factory->make($target->protocol_type);
        $credentials = $this->decryptCredentials($target);

        $config = [...($target->config_payload ?? []), 'host' => $target->host_or_endpoint, 'port' => $target->port, 'target_id' => $target->id];
        $adapter->forTarget($target)->configure($config, $credentials);

        $result = $adapter->execute();

        $this->recordLog($target, $result);
        $this->failures->record($target, $result, 'scheduled');
        $this->applyStateChange($target, $result);

        return $result;
    }

    /**
     * @return array<string, mixed>
     */
    private function decryptCredentials(MonitoringTarget $target): array
    {
        $credential = $target->credentials;

        if (! $credential) {
            return [];
        }

        try {
            return $this->encryption->decrypt(
                $credential->encrypted_payload,
                $credential->encryption_iv,
                $credential->encryption_tag,
            );
        } catch (\Throwable $e) {
            Log::warning('Credential decryption failed', ['target_id' => $target->id, 'error' => $e->getMessage()]);

            return [];
        }
    }

    private function recordLog(MonitoringTarget $target, ProbeResult $result): void
    {
        // Raw insert (not Eloquent) for partition performance.
        DB::table('monitoring_logs')->insert([
            'target_id' => $target->id,
            'status' => $result->status,
            'response_time_ms' => $result->responseTimeMs,
            'http_status_code' => $result->httpStatusCode,
            'error_message' => $result->errorMessage,
            'metrics_payload' => json_encode($result->metricsPayload),
            'checked_at' => now(),
        ]);
    }

    private function applyStateChange(MonitoringTarget $target, ProbeResult $result): void
    {
        $previous = $this->state->previousStatus($target->id);
        $isFailing = $result->isDown();

        if ($isFailing) {
            $failures = $this->state->incrementFailures($target->id);
            $threshold = $target->retry_threshold;

            // DOWN only after retry_threshold consecutive failures
            if ($failures >= $threshold && $previous !== 'DOWN') {
                $this->state->setStatus($target->id, 'DOWN', $result->responseTimeMs, $result->metricsPayload);
                $target->update(['current_status' => 'DOWN', 'last_checked_at' => now()]);

                $incident = Incident::create([
                    'target_id' => $target->id,
                    'severity' => 'CRITICAL',
                    'status' => 'OPEN',
                    'root_cause_message' => $result->errorMessage ?? 'Target unreachable',
                    'started_at' => now(),
                ]);

                // Inline: no queue worker consumes the alerts queue any more.
                ProcessStateChangeAlertJob::dispatchSync($target->id, 'DOWN', $incident->id);
            } else {
                $target->update(['last_checked_at' => now()]);
            }

            return;
        }

        // Success path
        $this->state->resetFailures($target->id);
        $this->state->setStatus($target->id, $result->status, $result->responseTimeMs, $result->metricsPayload);

        $target->update([
            'current_status' => $result->status,
            'last_checked_at' => now(),
        ]);

        // Recovery: previous was DOWN and now UP
        if ($previous === 'DOWN' && $result->isUp()) {
            $this->resolveOpenIncidents($target);
            ProcessStateChangeAlertJob::dispatchSync($target->id, 'RECOVERED');
        }
    }

    private function resolveOpenIncidents(MonitoringTarget $target): void
    {
        $openIncidents = Incident::where('target_id', $target->id)->open()->get();

        foreach ($openIncidents as $incident) {
            $resolvedAt = now();
            $incident->update([
                'status' => 'RESOLVED',
                'resolved_at' => $resolvedAt,
                // Whole seconds between start and resolve. Carbon 3's
                // diffInSeconds returns a signed float and the column is an
                // integer, so the epoch timestamps are subtracted instead.
                'duration_seconds' => max(0, $resolvedAt->getTimestamp() - $incident->started_at->getTimestamp()),
            ]);
        }
    }
}
