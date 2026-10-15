<?php

namespace App\Modules\Probes\Contracts;

/**
 * Value object returned by every probe adapter.
 * Mirrors ProbeResult in resources/js/types/monitoring.d.ts.
 */
final class ProbeResult
{
    public function __construct(
        public readonly string $status,
        public readonly ?float $responseTimeMs = null,
        public readonly ?string $errorMessage = null,
        public readonly array $metricsPayload = [],
        public readonly ?int $httpStatusCode = null,
    ) {
    }

    public static function up(?float $responseTimeMs = null, array $metrics = [], ?int $httpStatus = null): self
    {
        return new self('UP', $responseTimeMs, null, $metrics, $httpStatus);
    }

    public static function degraded(?float $responseTimeMs = null, array $metrics = [], ?string $reason = null): self
    {
        return new self('DEGRADED', $responseTimeMs, $reason, $metrics);
    }

    public static function down(?string $errorMessage = null, array $metrics = [], ?float $responseTimeMs = null): self
    {
        return new self('DOWN', $responseTimeMs, $errorMessage, $metrics);
    }

    public function isUp(): bool
    {
        return $this->status === 'UP';
    }

    public function isDown(): bool
    {
        return $this->status === 'DOWN';
    }

    public function toArray(): array
    {
        return [
            'status' => $this->status,
            'responseTimeMs' => $this->responseTimeMs,
            'errorMessage' => $this->errorMessage,
            'metricsPayload' => $this->metricsPayload,
            'httpStatusCode' => $this->httpStatusCode,
        ];
    }
}
