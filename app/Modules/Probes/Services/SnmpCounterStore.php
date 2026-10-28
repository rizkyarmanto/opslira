<?php

namespace App\Modules\Probes\Services;

use Illuminate\Support\Facades\Redis;

/**
 * Previous interface byte counters per target, so a probe can turn two absolute
 * readings into a rate. Keys:
 *  - target:snmp-counters:{target_id} -> JSON {ifIndex: {in: int, out: int, at: epoch}}
 *
 * State expires with the same TTL as TargetStateService: a probe that stops for
 * a week has no usable previous reading anyway.
 */
class SnmpCounterStore
{
    private const PREFIX = 'target:snmp-counters:';

    private const TTL = 604800; // 7 days

    /**
     * @return array<string, array{in: ?int, out: ?int, at: ?float}>
     */
    public function previous(string $targetId): array
    {
        $raw = Redis::get(self::PREFIX.$targetId);

        if (! $raw) {
            return [];
        }

        $decoded = json_decode($raw, true);

        return is_array($decoded) ? $decoded : [];
    }

    /**
     * @param  array<string, array{in: ?int, out: ?int}>  $counters
     */
    public function save(string $targetId, array $counters, float $readAt): void
    {
        $payload = [];

        foreach ($counters as $index => $counter) {
            $payload[$index] = [
                'in' => $counter['in'] ?? null,
                'out' => $counter['out'] ?? null,
                'at' => $readAt,
            ];
        }

        Redis::setex(self::PREFIX.$targetId, self::TTL, json_encode($payload));
    }
}
