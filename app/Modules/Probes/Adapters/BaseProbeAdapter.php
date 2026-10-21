<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeAdapterInterface;
use App\Modules\Targets\Models\MonitoringTarget;

abstract class BaseProbeAdapter implements ProbeAdapterInterface
{
    /** @var array<string, mixed> */
    protected array $config = [];

    /** @var array<string, mixed> */
    protected array $credentials = [];

    protected int $timeoutSeconds = 5;

    protected int $degradedThresholdMs = 1000;

    public function configure(array $configPayload, array $credentials = []): static
    {
        $this->config = $configPayload;
        $this->credentials = $credentials;

        return $this;
    }

    /**
     * Apply the timeout from a form payload. forTarget() covers saved targets;
     * the pre-save test route has no target yet, so it must set the timeout
     * explicitly instead of leaving the 5 s default.
     */
    public function withTimeoutSeconds(int $seconds): static
    {
        $this->timeoutSeconds = max(1, min(60, $seconds));

        return $this;
    }

    public function forTarget(MonitoringTarget $target): static
    {
        $this->timeoutSeconds = $target->timeout_seconds ?? 5;
        $this->degradedThresholdMs = $target->degraded_threshold_ms ?? 1000;

        return $this->configure($target->config_payload ?? []);
    }

    /**
     * Helper: classify a latency against the degraded threshold.
     */
    protected function classifyLatency(?float $latencyMs): string
    {
        if ($latencyMs === null) {
            return 'UP';
        }

        return $latencyMs > $this->degradedThresholdMs ? 'DEGRADED' : 'UP';
    }

    protected function config(string $key, mixed $default = null): mixed
    {
        return $this->config[$key] ?? $default;
    }

    protected function credential(string $key, mixed $default = null): mixed
    {
        return $this->credentials[$key] ?? $default;
    }
}
