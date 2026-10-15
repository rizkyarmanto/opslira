<?php

namespace App\Modules\Probes\Contracts;

use App\Modules\Targets\Models\MonitoringTarget;

/**
 * Contract for all probe adapters (Strategy pattern).
 * docs/gemini/dokumen_arsitektur_aplikasi_web.md:108-119
 */
interface ProbeAdapterInterface
{
    /**
     * Configure the adapter with a target's config_payload + credentials.
     *
     * @param  array<string, mixed>  $configPayload
     * @param  array<string, mixed>  $credentials  Decrypted credentials (may be empty)
     */
    public function configure(array $configPayload, array $credentials = []): static;

    /**
     * Execute the probe and return the result.
     */
    public function execute(): ProbeResult;

    /**
     * Whether this adapter supports the given protocol_type enum.
     */
    public function supportsProtocol(string $protocolType): bool;

    /**
     * Convenience: configure from a MonitoringTarget instance.
     */
    public function forTarget(MonitoringTarget $target): static;
}
