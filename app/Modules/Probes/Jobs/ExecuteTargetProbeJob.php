<?php

namespace App\Modules\Probes\Jobs;

use App\Modules\Probes\Services\ProbeExecutionService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Bus\Queueable;
use Illuminate\Contracts\Queue\ShouldQueue;
use Illuminate\Foundation\Bus\Dispatchable;
use Illuminate\Queue\InteractsWithQueue;
use Illuminate\Queue\SerializesModels;
use Illuminate\Support\Facades\Log;

/**
 * Executes a single target probe. Dispatched to queue:probes.
 * docs/gemini/dokumen_arsitektur_aplikasi_web.md:104, 158
 */
class ExecuteTargetProbeJob implements ShouldQueue
{
    use Dispatchable, InteractsWithQueue, Queueable, SerializesModels;

    public int $tries = 1;

    public int $timeout = 15;

    public function __construct(public string $targetId)
    {
        $this->onQueue('probes');
    }

    public function handle(ProbeExecutionService $service): void
    {
        $target = MonitoringTarget::find($this->targetId);

        if (! $target || ! $target->is_active || $target->is_maintenance) {
            return;
        }

        try {
            $service->execute($target);
        } catch (\Throwable $e) {
            Log::error('Probe execution failed', [
                'target_id' => $this->targetId,
                'error' => $e->getMessage(),
            ]);

            throw $e;
        }
    }
}
