<?php

namespace App\Modules\Probes\Services;

use App\Modules\Probes\Adapters\BeszelApiProbeAdapter;
use App\Modules\Probes\Adapters\DatabasePingProbeAdapter;
use App\Modules\Probes\Adapters\DomainSslProbeAdapter;
use App\Modules\Probes\Adapters\HttpHealthProbeAdapter;
use App\Modules\Probes\Adapters\MailServerProbeAdapter;
use App\Modules\Probes\Adapters\NetworkSnmpProbeAdapter;
use App\Modules\Probes\Adapters\S3BucketProbeAdapter;
use App\Modules\Probes\Contracts\ProbeAdapterInterface;
use InvalidArgumentException;

/**
 * Resolves a ProbeAdapterInterface implementation for a protocol_type.
 */
class ProbeAdapterFactory
{
    /** @var array<string, class-string<ProbeAdapterInterface>> */
    private array $map = [
        'ICMP_SNMP' => NetworkSnmpProbeAdapter::class,
        'HTTP_HEALTH' => HttpHealthProbeAdapter::class,
        'WHOIS_SSL' => DomainSslProbeAdapter::class,
        'DB_NATIVE_PING' => DatabasePingProbeAdapter::class,
        'SMTP_DNSBL' => MailServerProbeAdapter::class,
        'S3_HEAD' => S3BucketProbeAdapter::class,
        'BESZEL_API' => BeszelApiProbeAdapter::class,
    ];

    public function make(string $protocolType): ProbeAdapterInterface
    {
        if (! isset($this->map[$protocolType])) {
            throw new InvalidArgumentException("Unsupported protocol_type: {$protocolType}");
        }

        return app($this->map[$protocolType]);
    }

    /**
     * @return array<int, string>
     */
    public function supportedProtocols(): array
    {
        return array_keys($this->map);
    }
}
