<?php

namespace App\Modules\Probes\Services;

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Facades\Log;

/**
 * One warning line for every non-UP target check, so laravel.log names the
 * target, the phase, and the reason instead of only the happy path. Monitoring
 * rows carry history; this is the diagnostic trail, and every string is masked
 * before it is written so a credential never lands in a log file.
 */
class ProbeFailureLogger
{
    /** Longest message kept; adapter errors can carry an entire HTTP body. */
    private const MAX_LENGTH = 1000;

    /**
     * @param  array<string, mixed>  $identity  extra context: target_type, protocol_type, host
     */
    public function record(MonitoringTarget $target, ProbeResult $result, string $phase, array $identity = []): void
    {
        if ($result->status === 'UP') {
            return;
        }

        Log::warning('Target probe failed', [
            'target_id' => $target->id,
            'target_name' => self::mask((string) $target->name),
            'target_type' => $target->target_type,
            'protocol_type' => $target->protocol_type,
            'host' => self::mask((string) $target->host_or_endpoint),
            'phase' => $phase,
            'status' => $result->status,
            'error_message' => $result->errorMessage !== null ? self::mask($result->errorMessage) : null,
            'response_time_ms' => $result->responseTimeMs,
            'metrics_payload' => $this->maskArray($result->metricsPayload),
            ...$identity,
        ]);
    }

    /**
     * Replaces credential-shaped values with ****. Labels stay so the line still
     * says which field failed. Applied to the whole message, not to a known
     * credential list: an error string is built by the adapter and can quote a
     * request body, a PDO DSN, or an SNMP command line.
     */
    public static function mask(string $text): string
    {
        $masked = preg_replace(
            '/(Authorization:\s*)(?:Bearer\s+|Basic\s+)?\S+/i',
            '$1****',
            $text,
        ) ?? $text;

        $masked = preg_replace(
            '/((?:password|passwd|pwd|secret(?:_key)?|api[_-]?key|access[_-]?key|bearer_token|token|community|identity|auth_password|priv_password|username|user)\s*[:=]\s*)(?:"[^"]*"|\'[^\']*\'|[^\s@]+)/i',
            '$1****',
            $masked,
        ) ?? $masked;

        // scheme://user:pass@host credentials
        $masked = preg_replace('#(://[^/\s:@]+:)[^/\s@]+@#', '$1****@', $masked) ?? $masked;

        return mb_substr($masked, 0, self::MAX_LENGTH);
    }

    /**
     * @param  array<mixed>  $values
     * @return array<mixed>
     */
    private function maskArray(array $values): array
    {
        $masked = [];
        foreach ($values as $key => $value) {
            $masked[$key] = is_array($value)
                ? $this->maskArray($value)
                : (is_string($value) ? self::mask($value) : $value);
        }

        return $masked;
    }
}
