<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeFailureLogger;
use App\Modules\Probes\Services\SnmpCounterStore;
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
    /** SYSTEM-MIB scalars. */
    private const OID_SYS_DESCR = '1.3.6.1.2.1.1.1.0';

    private const OID_SYS_OBJECT_ID = '1.3.6.1.2.1.1.2.0';

    private const OID_SYS_UPTIME = '1.3.6.1.2.1.1.3.0';

    private const OID_SYS_NAME = '1.3.6.1.2.1.1.5.0';

    /** MikroTik health scalars; absent on other vendors. */
    private const OID_MTX_CPU_LOAD = '1.3.6.1.4.1.14988.1.1.3.11.0';

    private const OID_MTX_TEMPERATURE = '1.3.6.1.4.1.14988.1.1.3.10.0';

    private const OID_MTX_VOLTAGE = '1.3.6.1.4.1.14988.1.1.3.8.0';

    /** HOST-RESOURCES-MIB hrProcessorLoad, one row per core. */
    private const OID_HR_PROCESSOR_LOAD = '1.3.6.1.2.1.25.3.3.1.2';

    /** HOST-RESOURCES-MIB hrStorageTable columns. */
    private const OID_HR_STORAGE_DESCR = '1.3.6.1.2.1.25.2.3.1.3';

    private const OID_HR_STORAGE_ALLOC_UNITS = '1.3.6.1.2.1.25.2.3.1.4';

    private const OID_HR_STORAGE_SIZE = '1.3.6.1.2.1.25.2.3.1.5';

    private const OID_HR_STORAGE_USED = '1.3.6.1.2.1.25.2.3.1.6';

    /** POWER-ETHERNET-MIB pethPsePortTable base, indexed by group.port. */
    private const OID_PETH_PSE_PORT_BASE = '1.3.6.1.2.1.105.1.1.1.1';

    /** POWER-ETHERNET-MIB pethMainPsePower, per PSE group. */
    private const OID_PETH_MAIN_PSE_POWER = '1.3.6.1.2.1.105.1.3.1.1.2';

    /**
     * IF-MIB columns joined on ifIndex. ifName and the 64-bit counters are from
     * the ifXTable extension; a device that predates it keeps the 32-bit ones.
     */
    private const IF_COLUMNS = [
        'descr' => '1.3.6.1.2.1.2.2.1.2',
        'type' => '1.3.6.1.2.1.2.2.1.3',
        'admin' => '1.3.6.1.2.1.2.2.1.7',
        'oper' => '1.3.6.1.2.1.2.2.1.8',
        'in_octets' => '1.3.6.1.2.1.2.2.1.10',
        'out_octets' => '1.3.6.1.2.1.2.2.1.16',
        'name' => '1.3.6.1.2.1.31.1.1.1.1',
        'hc_in_octets' => '1.3.6.1.2.1.31.1.1.1.6',
        'hc_out_octets' => '1.3.6.1.2.1.31.1.1.1.10',
        'high_speed' => '1.3.6.1.2.1.31.1.1.1.15',
    ];

    /** IANAifType ppp: tunnels counted as PPP links in the port table. */
    private const IF_TYPE_PPP = 23;

    private const MAX_LISTED_PORTS = 64;

    /**
     * Ports read by the last collectInterfaces() call, so the router extras can
     * count PPP tunnels from the full table rather than the truncated listing.
     *
     * @var array<int, array<string, mixed>>
     */
    private array $lastPorts = [];

    /**
     * The counter store is optional so a unit test can build the adapter
     * without a Redis connection; without it throughput stays null.
     */
    public function __construct(private readonly ?SnmpCounterStore $counters = null)
    {
    }

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
        $credential = $this->snmpCredentialArgv();
        if ($credential === []) {
            return [];
        }

        return ['snmpget', ...$credential, '-t', (string) $this->timeoutSeconds, $host, self::OID_SYS_UPTIME];
    }

    /**
     * One snmpget for several OIDs, with the numeric-quick output the parser
     * expects. -r 1 keeps a dead agent from retrying for the whole timeout.
     *
     * @param  array<int, string>  $oids
     * @return array<int, string>
     */
    public function snmpGetArguments(string $host, array $oids): array
    {
        $credential = $this->snmpCredentialArgv();
        if ($credential === [] || $oids === []) {
            return [];
        }

        return [
            'snmpget', ...$credential,
            '-t', (string) $this->timeoutSeconds, '-r', '1', '-Oqn',
            $host, ...array_values($oids),
        ];
    }

    /**
     * @return array<int, string>
     */
    public function snmpWalkArguments(string $host, string $baseOid): array
    {
        $credential = $this->snmpCredentialArgv();
        if ($credential === []) {
            return [];
        }

        return [
            'snmpwalk', ...$credential,
            '-t', (string) $this->timeoutSeconds, '-r', '1', '-Oqn',
            $host, $baseOid,
        ];
    }

    /**
     * Community or USM flags shared by every net-snmp command, in the order the
     * tools expect them. Empty means "no SNMP credentials configured".
     *
     * @return array<int, string>
     */
    private function snmpCredentialArgv(): array
    {
        $community = $this->credential('community');
        if (is_string($community) && $community !== '') {
            return ['-v', '2c', '-c', $community];
        }

        $username = $this->credential('username');
        $authPassword = $this->credential('auth_password');
        if (is_string($username) && $username !== '' && is_string($authPassword) && $authPassword !== '') {
            return [
                '-v', '3', '-u', $username, '-l', 'authPriv',
                '-a', (string) $this->credential('auth_protocol', 'SHA'),
                '-A', $authPassword,
                '-x', (string) $this->credential('priv_protocol', 'AES'),
                '-X', (string) $this->credential('priv_password', ''),
            ];
        }

        return [];
    }

    /**
     * SNMP reading for a network device. Reachability first, then the identity and
     * resource block, then, for a router or switch, the port table and the
     * sub_type extras. A failed sub-walk leaves only its own keys null instead of
     * failing the probe: ICMP still decides UP/DEGRADED/DOWN.
     *
     * @return array<string, mixed>
     */
    private function snmpMetrics(string $host): array
    {
        if ($this->snmpCredentialArgv() === []) {
            return ['snmp_available' => false, 'snmp_skipped' => 'no credentials configured'];
        }

        $reachability = $this->runSnmp($this->snmpGetArguments($host, [self::OID_SYS_UPTIME]), $host);
        if (! $reachability['ok']) {
            return [
                'snmp_available' => false,
                'snmp_error' => $reachability['error'] ?? 'snmp command failed',
            ];
        }

        $metrics = ['snmp_available' => true, ...$this->collectIdentityAndResource($host)];

        $subType = $this->config('sub_type');
        $subType = is_string($subType) ? strtoupper($subType) : null;

        if (! in_array($subType, ['ROUTER', 'SWITCH'], true)) {
            return $metrics;
        }

        $metrics = [...$metrics, ...$this->collectInterfaces($host)];

        if ($subType === 'ROUTER') {
            return [...$metrics, ...$this->collectRouterExtras($host)];
        }

        return [...$metrics, ...$this->collectPoe($host)];
    }

    /**
     * Run one net-snmp command. Never throws: a failure comes back as ok=false
     * with the masked reason, so the caller decides what to do with it.
     *
     * @param  array<int, string>  $arguments
     * @return array{ok: bool, output: ?string, error: ?string}
     */
    private function runSnmp(array $arguments, string $host): array
    {
        if ($arguments === []) {
            return ['ok' => false, 'output' => null, 'error' => null];
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

                return ['ok' => false, 'output' => null, 'error' => ProbeFailureLogger::mask($reason)];
            }

            return ['ok' => true, 'output' => $process->getOutput(), 'error' => null];
        } catch (\Throwable $e) {
            Log::warning('SNMP poll threw', [
                'host' => $host,
                'command' => $this->redactedCommand($arguments),
                'error' => ProbeFailureLogger::mask($e->getMessage()),
            ]);

            return ['ok' => false, 'output' => null, 'error' => ProbeFailureLogger::mask($e->getMessage())];
        }
    }

    /**
     * Device identity plus CPU and memory. Every key stays null when the device
     * does not answer that OID, so the UI renders n/a instead of a zero nobody
     * measured.
     *
     * @return array<string, mixed>
     */
    private function collectIdentityAndResource(string $host): array
    {
        $values = $this->getValues($host, [
            self::OID_SYS_DESCR,
            self::OID_SYS_OBJECT_ID,
            self::OID_SYS_UPTIME,
            self::OID_SYS_NAME,
        ]);

        return [
            'device_sys_descr' => $this->snmpText($values[self::OID_SYS_DESCR] ?? null),
            'device_sys_name' => $this->snmpText($values[self::OID_SYS_NAME] ?? null),
            'device_sys_object_id' => $this->snmpText($values[self::OID_SYS_OBJECT_ID] ?? null),
            'device_uptime_seconds' => $this->snmpUptimeSeconds($values[self::OID_SYS_UPTIME] ?? null),
            'cpu_usage_pct' => $this->cpuUsage($host),
            ...$this->memoryUsage($host),
        ];
    }

    /**
     * MikroTik answers one CPU load scalar; the standard route is one
     * hrProcessorLoad row per core, averaged here.
     */
    private function cpuUsage(string $host): ?float
    {
        $values = $this->getValues($host, [self::OID_MTX_CPU_LOAD]);
        $vendor = $this->toFloat($values[self::OID_MTX_CPU_LOAD] ?? null);

        if ($vendor !== null) {
            return round($vendor, 1);
        }

        $loads = [];

        foreach ($this->walkColumn($host, self::OID_HR_PROCESSOR_LOAD) as $value) {
            $load = $this->toFloat($value);

            if ($load !== null) {
                $loads[] = $load;
            }
        }

        return $loads === [] ? null : round(array_sum($loads) / count($loads), 1);
    }

    /**
     * hrStorageTable: the first row whose description names memory. Used over size
     * gives the percentage; both scale by the row's allocation units for bytes.
     *
     * @return array{mem_usage_pct: ?float, mem_used_bytes: ?int, mem_total_bytes: ?int}
     */
    private function memoryUsage(string $host): array
    {
        $descr = $this->walkColumn($host, self::OID_HR_STORAGE_DESCR);
        $units = $this->walkColumn($host, self::OID_HR_STORAGE_ALLOC_UNITS);
        $size = $this->walkColumn($host, self::OID_HR_STORAGE_SIZE);
        $used = $this->walkColumn($host, self::OID_HR_STORAGE_USED);

        foreach ($descr as $index => $description) {
            if (preg_match('/memory|ram/i', (string) $description) !== 1) {
                continue;
            }

            $sizeValue = $this->toFloat($size[$index] ?? null);
            $usedValue = $this->toFloat($used[$index] ?? null);
            $unitsValue = $this->toFloat($units[$index] ?? null) ?? 1.0;

            if ($sizeValue === null || $sizeValue <= 0 || $usedValue === null) {
                continue;
            }

            return [
                'mem_usage_pct' => round($usedValue * 100 / $sizeValue, 1),
                'mem_used_bytes' => (int) round($usedValue * $unitsValue),
                'mem_total_bytes' => (int) round($sizeValue * $unitsValue),
            ];
        }

        return ['mem_usage_pct' => null, 'mem_used_bytes' => null, 'mem_total_bytes' => null];
    }

    /**
     * IF-MIB interface table joined on ifIndex, plus the throughput delta against
     * the previous probe. Returns no keys at all when the device answers no
     * interface column, which leaves the UI on n/a.
     *
     * @return array<string, mixed>
     */
    private function collectInterfaces(string $host): array
    {
        $columns = [];

        foreach (self::IF_COLUMNS as $key => $baseOid) {
            $columns[$key] = $this->walkColumn($host, $baseOid);
        }

        $indexes = [];

        foreach (['oper', 'admin', 'descr', 'name'] as $key) {
            foreach (array_keys($columns[$key]) as $index) {
                $indexes[(string) $index] = true;
            }
        }

        if ($indexes === []) {
            return [];
        }

        $indexList = array_map('intval', array_keys($indexes));
        sort($indexList);

        $counters = [];
        $ports = [];

        foreach ($indexList as $index) {
            $key = (string) $index;

            $counters[$key] = [
                'in' => $this->toFloat($columns['hc_in_octets'][$key] ?? null)
                    ?? $this->toFloat($columns['in_octets'][$key] ?? null),
                'out' => $this->toFloat($columns['hc_out_octets'][$key] ?? null)
                    ?? $this->toFloat($columns['out_octets'][$key] ?? null),
            ];

            $type = $columns['type'][$key] ?? null;

            $ports[$key] = [
                'if_index' => $index,
                'name' => $this->snmpText($columns['name'][$key] ?? null)
                    ?? $this->snmpText($columns['descr'][$key] ?? null)
                    ?? $key,
                'admin' => $this->interfaceStatus($columns['admin'][$key] ?? null),
                'oper' => $this->interfaceStatus($columns['oper'][$key] ?? null),
                'type' => $type === null ? null : (int) $this->toFloat($type),
                'speed_mbps' => $this->toFloat($columns['high_speed'][$key] ?? null),
                'rx_mbps' => null,
                'tx_mbps' => null,
            ];
        }

        foreach ($this->interfaceRates($counters) as $key => $rate) {
            $ports[$key]['rx_mbps'] = $rate['rx_mbps'];
            $ports[$key]['tx_mbps'] = $rate['tx_mbps'];
        }

        $this->lastPorts = array_values($ports);

        $measured = array_filter(
            $ports,
            fn (array $port) => $port['rx_mbps'] !== null || $port['tx_mbps'] !== null,
        );

        $rxTotal = 0.0;
        $txTotal = 0.0;

        foreach ($measured as $port) {
            $rxTotal += (float) $port['rx_mbps'];
            $txTotal += (float) $port['tx_mbps'];
        }

        return [
            'ports_total' => count($ports),
            'ports_up' => count(array_filter($ports, fn (array $port) => $port['oper'] === 'up')),
            'ports_down' => count(array_filter($ports, fn (array $port) => $port['oper'] === 'down')),
            'rx_mbps' => round($rxTotal, 3),
            'tx_mbps' => round($txTotal, 3),
            'ports' => array_slice($this->lastPorts, 0, self::MAX_LISTED_PORTS),
            'ports_truncated' => count($ports) > self::MAX_LISTED_PORTS,
        ];
    }

    /**
     * Rate per interface from the stored previous reading, then store this one.
     * Without a target id or a counter store (unit test, pre-save test) nothing is
     * cached and every rate stays null.
     *
     * @param  array<string, array{in: ?float, out: ?float}>  $counters
     * @return array<string, array{rx_mbps: ?float, tx_mbps: ?float}>
     */
    private function interfaceRates(array $counters): array
    {
        $targetId = $this->config('target_id');
        $hasStore = is_string($targetId) && $targetId !== '' && $this->counters !== null;

        $previous = $hasStore ? $this->counters->previous($targetId) : [];
        $now = microtime(true);

        $rates = $this->counterDeltas($previous, $counters, $now);

        if ($hasStore) {
            $this->counters->save($targetId, $counters, $now);
        }

        return $rates;
    }

    /**
     * Two absolute counter readings into a rate. Pure, so a unit test drives it
     * without a device. A counter that reads lower than before is a reset, not a
     * negative rate, so it yields null for that interface and direction.
     *
     * @param  array<string, array{in?: ?float, out?: ?float, at?: ?float}>  $previous
     * @param  array<string, array{in: ?float, out: ?float}>  $current
     * @return array<string, array{rx_mbps: ?float, tx_mbps: ?float}>
     */
    public function counterDeltas(array $previous, array $current, float $now): array
    {
        $rates = [];

        foreach ($current as $index => $counter) {
            $before = $previous[$index] ?? null;
            $before = is_array($before) ? $before : null;
            $elapsed = $before === null ? 0.0 : $now - (float) ($before['at'] ?? 0);

            $rates[$index] = [
                'rx_mbps' => $this->counterRate($counter['in'] ?? null, $before['in'] ?? null, $elapsed),
                'tx_mbps' => $this->counterRate($counter['out'] ?? null, $before['out'] ?? null, $elapsed),
            ];
        }

        return $rates;
    }

    private function counterRate(?float $current, ?float $previous, float $elapsedSeconds): ?float
    {
        if ($current === null || $previous === null || $elapsedSeconds < 1.0 || $current < $previous) {
            return null;
        }

        return round(($current - $previous) * 8 / $elapsedSeconds / 1_000_000, 3);
    }

    private function interfaceStatus(?string $value): ?string
    {
        $numeric = $this->toFloat($value);

        if ($numeric === null) {
            return null;
        }

        return (int) $numeric === 1 ? 'up' : 'down';
    }

    /**
     * Router extras: MikroTik health scalars plus the PPP tunnel count taken from
     * the port table (IANAifType 23), which is vendor neutral. Temperature and
     * voltage arrive in tenths.
     *
     * @return array<string, mixed>
     */
    private function collectRouterExtras(string $host): array
    {
        $values = $this->getValues($host, [self::OID_MTX_TEMPERATURE, self::OID_MTX_VOLTAGE]);

        $temperature = $this->toFloat($values[self::OID_MTX_TEMPERATURE] ?? null);
        $voltage = $this->toFloat($values[self::OID_MTX_VOLTAGE] ?? null);

        $tunnels = array_filter($this->lastPorts, fn (array $port) => $port['type'] === self::IF_TYPE_PPP);

        return [
            'sensor_temp_c' => $temperature === null ? null : round($temperature / 10, 1),
            'sensor_voltage_v' => $voltage === null ? null : round($voltage / 10, 1),
            'sensor_supported' => $temperature !== null || $voltage !== null,
            'ppp_tunnels_total' => $this->lastPorts === [] ? null : count($tunnels),
            'ppp_tunnels_up' => $this->lastPorts === []
                ? null
                : count(array_filter($tunnels, fn (array $port) => $port['oper'] === 'up')),
        ];
    }

    /**
     * POWER-ETHERNET-MIB PoE ports for a switch. The standard MIB has no
     * per-port wattage column, so per-port watts stay null. Some switches (an HP
     * 2530 among them) answer pethMainPsePower but not the per-port table, so a
     * readable PSE power alone still means "this device reports PoE": the ports
     * list stays empty and the UI says per-port state is unavailable. Only a
     * device that answers neither is reported as having no PoE.
     *
     * @return array<string, mixed>
     */
    private function collectPoe(string $host): array
    {
        $admin = $this->walkColumn($host, self::OID_PETH_PSE_PORT_BASE.'.3', true);
        $detection = $this->walkColumn($host, self::OID_PETH_PSE_PORT_BASE.'.6', true);
        $budget = $this->poeTotalWatts($host);

        if ($admin === [] && $detection === [] && $budget === null) {
            return [
                'poe_supported' => false,
                'poe_ports_total' => 0,
                'poe_ports_powered' => 0,
                'poe_total_watts' => null,
                'poe_ports' => [],
                'poe_truncated' => false,
            ];
        }

        $indexes = [];

        foreach ([$admin, $detection] as $column) {
            foreach (array_keys($column) as $index) {
                $indexes[(string) $index] = true;
            }
        }

        $keys = array_keys($indexes);
        sort($keys, SORT_NATURAL);

        $ports = [];
        $powered = 0;

        foreach ($keys as $index) {
            $detectionValue = $this->toFloat($detection[$index] ?? null);

            if ($detectionValue !== null && (int) $detectionValue === 3) {
                $powered++;
            }

            $ports[] = [
                'pse' => $index,
                'admin' => $this->poeAdminState($admin[$index] ?? null),
                'detection' => $this->poeDetectionState($detectionValue),
                'watts' => null,
            ];
        }

        return [
            'poe_supported' => true,
            'poe_ports_total' => count($ports),
            'poe_ports_powered' => $powered,
            'poe_total_watts' => $budget,
            'poe_ports' => array_slice($ports, 0, self::MAX_LISTED_PORTS),
            'poe_truncated' => count($ports) > self::MAX_LISTED_PORTS,
        ];
    }

    /** pethPsePortAdminEnable is a TruthValue: 1 true, 2 false. */
    private function poeAdminState(?string $value): ?string
    {
        $numeric = $this->toFloat($value);

        return match ($numeric === null ? null : (int) $numeric) {
            1 => 'on',
            2 => 'off',
            default => null,
        };
    }

    /** pethPsePortDetectionStatus: 1 disabled, 2 searching, 3 delivering, 4 fault. */
    private function poeDetectionState(?float $value): ?string
    {
        return match ($value === null ? null : (int) $value) {
            1 => 'disabled',
            2 => 'searching',
            3 => 'delivering',
            4 => 'fault',
            5 => 'test',
            6 => 'other_fault',
            default => null,
        };
    }

    /**
     * pethMainPsePower is the PSE's nominal power, so this is the PoE budget the
     * switch can supply, summed when several PSE groups answer.
     */
    private function poeTotalWatts(string $host): ?float
    {
        $watts = [];

        foreach ($this->walkColumn($host, self::OID_PETH_MAIN_PSE_POWER) as $value) {
            $numeric = $this->toFloat($value);

            if ($numeric !== null) {
                $watts[] = $numeric;
            }
        }

        return $watts === [] ? null : round(array_sum($watts), 1);
    }

    /**
     * One SNMP walk keyed by the table index suffix of each returned OID. A nested
     * table (pethPsePortTable) keeps the full group.port suffix.
     *
     * @return array<string, string>
     */
    private function walkColumn(string $host, string $baseOid, bool $nested = false): array
    {
        $probe = $this->runSnmp($this->snmpWalkArguments($host, $baseOid), $host);

        if (! $probe['ok'] || $probe['output'] === null) {
            return [];
        }

        $values = [];

        foreach ($this->parseSnmpLines($probe['output']) as $oid => $value) {
            if (! str_starts_with($oid, $baseOid.'.')) {
                continue;
            }

            $suffix = substr($oid, strlen($baseOid) + 1);

            if ($suffix === '' || (! $nested && str_contains($suffix, '.'))) {
                continue;
            }

            $values[$suffix] = $value;
        }

        return $values;
    }

    /**
     * @param  array<int, string>  $oids
     * @return array<string, string>
     */
    private function getValues(string $host, array $oids): array
    {
        $probe = $this->runSnmp($this->snmpGetArguments($host, $oids), $host);

        return $probe['ok'] && $probe['output'] !== null ? $this->parseSnmpLines($probe['output']) : [];
    }

    /**
     * net-snmp output, one "<oid> <TYPE>: <value>" line per varbind. The leading
     * dot is stripped so keys match the OID literals above, and the type prefix is
     * dropped: only the value matters here. Quick-mode output without a type
     * prefix parses the same way.
     *
     * @return array<string, string>
     */
    public function parseSnmpLines(string $output): array
    {
        $values = [];

        foreach (preg_split('/\r?\n/', $output) ?: [] as $line) {
            $line = trim($line);

            if ($line === '' || preg_match('/^(\.\S+)\s+(.*)$/', $line, $matches) !== 1) {
                continue;
            }

            $value = trim($matches[2]);

            if (preg_match('/^[A-Za-z][A-Za-z0-9-]*:\s*(.*)$/', $value, $typed) === 1) {
                $value = trim($typed[1]);
            }

            $values[ltrim($matches[1], '.')] = $value;
        }

        return $values;
    }

    private function snmpText(?string $value): ?string
    {
        if ($value === null) {
            return null;
        }

        $trimmed = trim($value, " \t\n\r\0\x0B\"");

        return $trimmed === '' ? null : $trimmed;
    }

    private function toFloat(?string $value): ?float
    {
        if ($value === null || preg_match('/-?\d+(?:\.\d+)?/', $value, $matches) !== 1) {
            return null;
        }

        return (float) $matches[0];
    }

    /**
     * sysUpTime is a TimeTicks counter in hundredths of a second. net-snmp prints
     * "Timeticks: (12345) 0:02:03.45"; quick mode prints just the tick count.
     */
    public function snmpUptimeSeconds(?string $value): ?int
    {
        if ($value === null) {
            return null;
        }

        if (preg_match('/\((\d+)\)/', $value, $matches) === 1) {
            return intdiv((int) $matches[1], 100);
        }

        $raw = $this->toFloat($value);

        return $raw === null ? null : intdiv((int) $raw, 100);
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
