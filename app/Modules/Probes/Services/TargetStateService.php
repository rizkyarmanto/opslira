<?php

namespace App\Modules\Probes\Services;

use App\Modules\Probes\Contracts\ProbeResult;
use Illuminate\Support\Facades\Redis;

/**
 * Redis state structure: docs/gemini/dokumen_arsitektur_aplikasi_web.md:128-130.
 *  - target:state:{uuid}    -> JSON status snapshot
 *  - target:failures:{id}   -> consecutive failure counter
 */
class TargetStateService
{
    private const STATE_PREFIX = 'target:state:';

    private const FAILURE_PREFIX = 'target:failures:';

    private const STATE_TTL = 604800; // 7 days

    /**
     * @return array{status: string, last_latency_ms: ?float, consecutive_failures: int, last_checked_at: ?string, summary_metrics: array}
     */
    public function get(string $targetId): array
    {
        $raw = Redis::get(self::STATE_PREFIX.$targetId);

        if (! $raw) {
            return [
                'status' => 'PENDING',
                'last_latency_ms' => null,
                'consecutive_failures' => 0,
                'last_checked_at' => null,
                'summary_metrics' => [],
            ];
        }

        return json_decode($raw, true) ?: [];
    }

    public function setStatus(string $targetId, string $status, ?float $latencyMs = null, array $metrics = []): void
    {
        $state = $this->get($targetId);
        $state['status'] = $status;
        $state['last_latency_ms'] = $latencyMs;
        $state['last_checked_at'] = now()->toIso8601String();
        $state['summary_metrics'] = $metrics;

        Redis::setex(self::STATE_PREFIX.$targetId, self::STATE_TTL, json_encode($state));
    }

    public function incrementFailures(string $targetId): int
    {
        $key = self::FAILURE_PREFIX.$targetId;
        $count = (int) Redis::incr($key);
        Redis::expire($key, self::STATE_TTL);

        $state = $this->get($targetId);
        $state['consecutive_failures'] = $count;
        Redis::setex(self::STATE_PREFIX.$targetId, self::STATE_TTL, json_encode($state));

        return $count;
    }

    public function resetFailures(string $targetId): void
    {
        Redis::del(self::FAILURE_PREFIX.$targetId);

        $state = $this->get($targetId);
        $state['consecutive_failures'] = 0;
        Redis::setex(self::STATE_PREFIX.$targetId, self::STATE_TTL, json_encode($state));
    }

    public function failures(string $targetId): int
    {
        return (int) (Redis::get(self::FAILURE_PREFIX.$targetId) ?? 0);
    }

    public function previousStatus(string $targetId): string
    {
        return $this->get($targetId)['status'] ?? 'PENDING';
    }
}
