<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeFailureLogger;
use Illuminate\Support\Facades\Log;
use Spatie\Ping\Ping;
use Spatie\Ping\PingResult;
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
        return in_array($protocolType, ['ICMP_SNMP', 'ICMP_PING'], true);
    }

    public function execute(): ProbeResult
    {
        $host = $this->config('host');
        if (! $host) {
            return ProbeResult::down('Missing host in config_payload');
        }

        // An empty form field stores null; without this the cast produced 0 and
        // the ping ran with a count the OS rejects. Absent or invalid means the
        // schema default of 4 packets.
        $rawCount = $this->config('icmp_count');
        $count = is_numeric($rawCount) ? max(1, (int) $rawCount) : 4;
        // INTERNET targets are ping only: no TCP fallback, no SNMP reading.
        $icmpOnly = $this->config('icmp_only', false) === true;
        $start = microtime(true);

        $icmp = $this->ping($host, $count);
        $latency = $icmp['avg_ms'];

        if (! $icmp['success']) {
            $detail = $icmp['raw'] ?? null;
            $message = sprintf(
                'Ping failed (%s): %s',
                $icmp['error'] ?? 'no reason reported',
                $detail ?: 'no output from ping',
            );

            if ($icmpOnly) {
                return ProbeResult::down($message);
            }

            // Fallback: TCP connect check when ICMP is unavailable
            $tcp = $this->tcpFallback($host);
            if (! $tcp['success']) {
                return ProbeResult::down($message);
            }

            return ProbeResult::up(
                $tcp['latency_ms'],
                ['packet_loss_pct' => null, 'icmp_available' => false, 'tcp_fallback' => true, 'icmp_error' => $icmp['error'] ?? null],
            );
        }

        $metrics = [
            'packet_loss_pct' => $icmp['packet_loss'],
            'jitter_ms' => $icmp['jitter'],
            'icmp_available' => true,
        ];

        // Optional SNMP metrics
        if (! $icmpOnly && (bool) $this->config('enable_snmp', false)) {
            $metrics = [...$metrics, ...$this->snmpMetrics($host)];
        }

        $status = $this->classifyLatency($latency);

        return new ProbeResult($status, $latency, null, $metrics);
    }

    /**
     * Map a spatie/ping result onto the adapter's stats shape. The package
     * reports loss as an int percentage and jitter as the stddev of timings;
     * a failed ping has no timings, so avg_ms and jitter stay null. 'raw' keeps
     * the ping stdout, which is the only place a rejected command line shows up.
     *
     * @return array{success: bool, avg_ms: ?float, packet_loss: ?float, jitter: ?float, error: ?string, raw: ?string}
     */
    public function pingResultToStats(PingResult $result): array
    {
        if (! $result->isSuccess()) {
            return [
                'success' => false,
                'avg_ms' => null,
                'packet_loss' => (float) $result->packetLossPercentage(),
                'jitter' => null,
                'error' => $result->error()?->value ?? 'ping failed',
                'raw' => $this->pingFailureDetail($result),
            ];
        }

        return [
            'success' => true,
            'avg_ms' => $result->averageTimeInMs(),
            'packet_loss' => (float) $result->packetLossPercentage(),
            'jitter' => $result->standardDeviationTimeInMs(),
            'error' => null,
            'raw' => null,
        ];
    }

    /**
     * Windows ping.exe has no -c/-W/-O flags, so the package's argv is rejected
     * there. Returns null on other systems to signal "use the package".
     *
     * @return array<int, string>|null
     */
    public function pingCommandArguments(string $host, int $count): ?array
    {
        if (PHP_OS_FAMILY !== 'Windows') {
            return null;
        }

        return ['ping', '-n', (string) $count, '-w', (string) ($this->timeoutSeconds * 1000), $host];
    }

    /**
     * The raw ping stdout, trimmed and bounded, so a DOWN message carries the
     * actual reason (a rejected option, a DNS failure) instead of just a tag.
     */
    public function pingFailureDetail(PingResult $result): ?string
    {
        $raw = trim($result->rawOutput());

        return $raw === '' ? null : mb_substr($raw, 0, 500);
    }

    /**
     * Windows prints "Minimum = 0ms, Maximum = 1ms, Average = 0ms"; the package
     * only understands the Linux "min/avg/max/mdev" line.
     */
    public function windowsAverageTimeInMs(PingResult $result): ?float
    {
        preg_match('/Average = ([\d.]+)ms/i', $result->rawOutput(), $matches);

        return isset($matches[1]) ? (float) $matches[1] : null;
    }

    /**
     * Windows prints "Lost = 3 (75% loss)". Returns null when the line is
     * absent, so the caller keeps the parsed value instead of inventing one.
     */
    public function windowsPacketLossPercentage(PingResult $result): ?float
    {
        preg_match('/Lost = \d+ \(([\d.]+)% loss\)/i', $result->rawOutput(), $matches);

        return isset($matches[1]) ? (float) $matches[1] : null;
    }

    /**
     * Reads a Windows ping failure from its own wording, so the result carries
     * "host not found" or "request timed out" instead of "unknownError".
     */
    public function windowsPingError(PingResult $result): ?string
    {
        $output = strtolower($result->rawOutput());

        return match (true) {
            str_contains($output, 'could not find host') || str_contains($output, 'cannot find host') => 'host not found',
            str_contains($output, 'destination host unreachable') || str_contains($output, 'no route to host') => 'host unreachable',
            str_contains($output, 'request timed out') || str_contains($output, 'request timeout') => 'request timed out',
            str_contains($output, 'general failure') => 'general failure',
            default => null,
        };
    }

    /**
     * @return array{success: bool, avg_ms: ?float, packet_loss: ?float, jitter: ?float, error: ?string, raw: ?string}
     */
    private function ping(string $host, int $count): array
    {
        try {
            if ($arguments = $this->pingCommandArguments($host, $count)) {
                $process = new Process($arguments);
                $process->setTimeout($this->timeoutSeconds * max(1, $count) + 5);
                $process->run();
                $lines = explode("\n", $process->getOutput()."\n".$process->getErrorOutput());

                $result = PingResult::fromPingOutput($lines, (int) $process->getExitCode(), $host, $this->timeoutSeconds);
                $stats = $this->pingResultToStats($result);

                // The package only parses Linux statistics, so on Windows the
                // average and loss are read from its own output format instead
                // of falling back to a null average and a claimed 0% loss.
                if ($stats['success']) {
                    $stats['avg_ms'] = $stats['avg_ms'] ?? $this->windowsAverageTimeInMs($result);
                    $stats['packet_loss'] = $this->windowsPacketLossPercentage($result) ?? $stats['packet_loss'];
                }

                // Its error needles are Linux strings, so a failed Windows ping
                // reports the useless "unknownError" tag. Classify it here.
                if ($stats['error'] === 'unknownError' && ($windowsError = $this->windowsPingError($result)) !== null) {
                    $stats['error'] = $windowsError;
                }

                return $stats;
            }

            $result = (new Ping($host, timeoutInSeconds: $this->timeoutSeconds, count: $count))->run();

            return $this->pingResultToStats($result);
        } catch (\Throwable $e) {
            return ['success' => false, 'avg_ms' => null, 'packet_loss' => null, 'jitter' => null, 'error' => $e->getMessage(), 'raw' => null];
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

    /**
     * Build the snmpget argv from the stored credential. Version is driven by the
     * credential payload, not config_payload, and there is no silent 'public'
     * community fallback: an empty array means "no SNMP credentials configured".
     *
     * @return array<int, string>
     */
    public function snmpCommandArguments(string $host): array
    {
        $tail = ['-t', (string) $this->timeoutSeconds, $host, '1.3.6.1.2.1.1.3.0'];

        $community = $this->credential('community');
        if (is_string($community) && $community !== '') {
            return ['snmpget', '-v', '2c', '-c', $community, ...$tail];
        }

        $username = $this->credential('username');
        $authPassword = $this->credential('auth_password');
        if (is_string($username) && $username !== '' && is_string($authPassword) && $authPassword !== '') {
            $authProtocol = (string) $this->credential('auth_protocol', 'SHA');
            $privProtocol = (string) $this->credential('priv_protocol', 'AES');
            $privPassword = (string) $this->credential('priv_password', '');

            return [
                'snmpget', '-v', '3', '-u', $username, '-l', 'authPriv',
                '-a', $authProtocol, '-A', $authPassword,
                '-x', $privProtocol, '-X', $privPassword,
                ...$tail,
            ];
        }

        return [];
    }

    private function snmpMetrics(string $host): array
    {
        $arguments = $this->snmpCommandArguments($host);
        if ($arguments === []) {
            return ['snmp_available' => false, 'snmp_skipped' => 'no credentials configured'];
        }

        try {
            $process = new Process($arguments);
            $process->setTimeout($this->timeoutSeconds + 2);
            $process->run();

            if (! $process->isSuccessful()) {
                $stderr = trim($process->getErrorOutput());
                $reason = $stderr !== '' ? $stderr : 'exit code '.$process->getExitCode();

                Log::warning('SNMP poll failed', [
                    'host' => $host,
                    'command' => $this->redactedCommand($arguments),
                    'error' => ProbeFailureLogger::mask($reason),
                ]);

                return ['snmp_available' => false, 'snmp_error' => ProbeFailureLogger::mask($reason)];
            }

            return ['snmp_available' => true];
        } catch (\Throwable $e) {
            Log::warning('SNMP poll threw', [
                'host' => $host,
                'command' => $this->redactedCommand($arguments),
                'error' => ProbeFailureLogger::mask($e->getMessage()),
            ]);

            return ['snmp_available' => false, 'snmp_error' => ProbeFailureLogger::mask($e->getMessage())];
        }
    }

    /**
     * Command line for the log with the values after -c/-A/-X (community, auth
     * password, priv password) replaced, since those arrive as bare argv items
     * with no key= separator for mask() to catch.
     *
     * @param  array<int, string>  $arguments
     */
    private function redactedCommand(array $arguments): string
    {
        $secretFlags = ['-c', '-A', '-X'];
        $redacted = [];

        foreach ($arguments as $index => $value) {
            $previous = $arguments[$index - 1] ?? null;
            $redacted[] = in_array($previous, $secretFlags, true) ? '****' : $value;
        }

        return implode(' ', $redacted);
    }
}
