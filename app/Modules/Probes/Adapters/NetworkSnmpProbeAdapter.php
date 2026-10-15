<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use Illuminate\Support\Facades\Log;
use Symfony\Component\Process\Process;

/**
 * NETWORK_DEVICE: ICMP Ping (+ optional SNMP OID polling).
 * Requires iputils-ping + net-snmp binaries in the container.
 * ICMP needs CAP_NET_RAW (docker-compose cap_add).
 */
class NetworkSnmpProbeAdapter extends BaseProbeAdapter
{
    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'ICMP_SNMP';
    }

    public function execute(): ProbeResult
    {
        $host = $this->config('host');
        if (! $host) {
            return ProbeResult::down('Missing host in config_payload');
        }

        $count = (int) $this->config('icmp_count', 4);
        $start = microtime(true);

        $icmp = $this->ping($host, $count);
        $latency = $icmp['avg_ms'];

        if (! $icmp['success']) {
            // Fallback: TCP connect check when ICMP is unavailable
            $tcp = $this->tcpFallback($host);
            if (! $tcp['success']) {
                return ProbeResult::down($icmp['error'] ?? 'ICMP + TCP fallback failed');
            }

            return ProbeResult::up(
                $tcp['latency_ms'],
                ['packet_loss_pct' => null, 'icmp_available' => false, 'tcp_fallback' => true],
            );
        }

        $metrics = [
            'packet_loss_pct' => $icmp['packet_loss'],
            'jitter_ms' => $icmp['jitter'],
            'icmp_available' => true,
        ];

        // Optional SNMP metrics
        if ((bool) $this->config('enable_snmp', false)) {
            $metrics = [...$metrics, ...$this->snmpMetrics($host)];
        }

        $status = $this->classifyLatency($latency);

        return new ProbeResult($status, $latency, null, $metrics);
    }

    /**
     * @return array{success: bool, avg_ms: ?float, packet_loss: ?float, jitter: ?float, error: ?string}
     */
    private function ping(string $host, int $count): array
    {
        try {
            $process = new Process(['ping', '-c', (string) $count, '-W', (string) $this->timeoutSeconds, $host]);
            $process->setTimeout($this->timeoutSeconds + 2);
            $process->run();

            if (! $process->isSuccessful()) {
                return ['success' => false, 'avg_ms' => null, 'packet_loss' => 100.0, 'jitter' => null, 'error' => trim($process->getErrorOutput()) ?: 'ping failed'];
            }

            $output = $process->getOutput();
            preg_match('/=\s*([\d.]+)\/([\d.]+)\/([\d.]+)\/([\d.]+)/', $output, $m);
            preg_match('/([\d.]+)% packet loss/', $output, $l);

            return [
                'success' => true,
                'avg_ms' => isset($m[2]) ? (float) $m[2] : null,
                'packet_loss' => isset($l[1]) ? (float) $l[1] : 0.0,
                'jitter' => isset($m[4]) ? (float) $m[4] : null,
                'error' => null,
            ];
        } catch (\Throwable $e) {
            return ['success' => false, 'avg_ms' => null, 'packet_loss' => null, 'jitter' => null, 'error' => $e->getMessage()];
        }
    }

    /**
     * @return array{success: bool, latency_ms: ?float}
     */
    private function tcpFallback(string $host): array
    {
        $port = (int) $this->config('port', 0);
        if ($port === 0) {
            return ['success' => false, 'latency_ms' => null];
        }

        $start = microtime(true);
        $conn = @fsockopen($host, $port, $errno, $errstr, $this->timeoutSeconds);
        $latency = (microtime(true) - $start) * 1000;

        if ($conn) {
            fclose($conn);

            return ['success' => true, 'latency_ms' => round($latency, 2)];
        }

        return ['success' => false, 'latency_ms' => null];
    }

    private function snmpMetrics(string $host): array
    {
        // Minimal SNMP v2c poll; extend with full OID set as needed.
        $community = (string) $this->credential('community', 'public');

        try {
            $process = new Process(['snmpget', '-v', '2c', '-c', $community, '-t', (string) $this->timeoutSeconds, $host, '1.3.6.1.2.1.1.3.0']);
            $process->setTimeout($this->timeoutSeconds + 2);
            $process->run();

            if (! $process->isSuccessful()) {
                return ['snmp_available' => false];
            }

            return ['snmp_available' => true];
        } catch (\Throwable $e) {
            Log::debug('SNMP poll failed', ['host' => $host, 'error' => $e->getMessage()]);

            return ['snmp_available' => false];
        }
    }
}
