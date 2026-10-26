<?php

namespace App\Modules\Probes\Adapters;

use App\Modules\Probes\Contracts\ProbeResult;
use PDO;

/**
 * DATABASE: TCP handshake + native driver 'SELECT 1'.
 */
class DatabasePingProbeAdapter extends BaseProbeAdapter
{
    public function supportsProtocol(string $protocolType): bool
    {
        return $protocolType === 'DB_NATIVE_PING';
    }

    public function execute(): ProbeResult
    {
        $host = $this->config('host');
        $port = (int) $this->config('port', 5432);
        $driver = (string) $this->config('db_driver', 'pgsql');
        $database = (string) $this->config('database_name', '');
        $query = (string) $this->config('query', 'SELECT 1');

        if (! $host) {
            return ProbeResult::down('Missing host in config_payload');
        }

        $username = (string) $this->credential('username', '');
        $password = (string) $this->credential('password', '');

        $start = microtime(true);

        try {
            $dsn = match ($driver) {
                'mysql', 'mariadb' => "mysql:host={$host};port={$port};dbname={$database}",
                'sqlsrv' => "sqlsrv:Server={$host},{$port};Database={$database}",
                default => "pgsql:host={$host};port={$port};dbname={$database}",
            };

            $pdo = new PDO($dsn, $username, $password, [
                PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
                PDO::ATTR_TIMEOUT => $this->timeoutSeconds,
            ]);

            $tcpHandshake = round((microtime(true) - $start) * 1000, 2);

            $queryStart = microtime(true);
            $stmt = $pdo->query($query);
            $stmt?->fetchAll();
            $queryExec = round((microtime(true) - $queryStart) * 1000, 2);

            $metrics = [
                'tcp_handshake_ms' => $tcpHandshake,
                'query_exec_ms' => $queryExec,
            ];

            if (filter_var($this->config('collect_connection_stats', true), FILTER_VALIDATE_BOOLEAN)) {
                $metrics = [...$metrics, ...$this->connectionStats($pdo, $driver)];
            }

            // The form sends "true"/"false" as text; a plain (bool) cast would
            // turn the string "false" on.
            $checkReplicationLag = filter_var($this->config('check_replication_lag', false), FILTER_VALIDATE_BOOLEAN);
            if ($checkReplicationLag) {
                $metrics = [...$metrics, ...$this->replicationLag($pdo, $driver)];
            }

            $pdo = null;
            $latency = round((microtime(true) - $start) * 1000, 2);

            return new ProbeResult($this->classifyLatency($latency), $latency, null, $metrics);
        } catch (\Throwable $e) {
            return ProbeResult::down('DB probe error: '.$e->getMessage());
        }
    }

    private function connectionStats(PDO $pdo, string $driver): array
    {
        try {
            if ($driver === 'pgsql') {
                $active = $pdo->query('SELECT count(*) FROM pg_stat_activity')->fetchColumn();
                $max = $pdo->query('SHOW max_connections')->fetchColumn();

                return ['active_connections' => (int) $active, 'max_connections' => (int) $max];
            }

            if (in_array($driver, ['mysql', 'mariadb'], true)) {
                $active = $pdo->query("SHOW STATUS LIKE 'Threads_connected'")->fetchColumn(1);
                $max = $pdo->query("SHOW VARIABLES LIKE 'max_connections'")->fetchColumn(1);

                return ['active_connections' => (int) $active, 'max_connections' => (int) $max];
            }

            if ($driver === 'sqlsrv') {
                $active = $pdo->query('SELECT COUNT(*) FROM sys.dm_exec_sessions WHERE is_user_process = 1')->fetchColumn();
                $max = $pdo->query('SELECT @@MAX_CONNECTIONS')->fetchColumn();

                return ['active_connections' => (int) $active, 'max_connections' => (int) $max];
            }
        } catch (\Throwable $e) {
        }

        return [];
    }

    /**
     * Best-effort replication lag in seconds. Metric only; a threshold-driven
     * status change is not specified here so it stays out of classification.
     *
     * @return array<string, int>
     */
    private function replicationLag(PDO $pdo, string $driver): array
    {
        try {
            if ($driver === 'pgsql') {
                // Replay lag on a standby; returns 0 on a primary.
                $lag = $pdo->query(
                    'SELECT COALESCE(CAST(EXTRACT(EPOCH FROM (now() - pg_last_xact_replay_timestamp())) AS int), 0)'
                )->fetchColumn();

                return ['replication_lag_seconds' => (int) $lag];
            }

            if (in_array($driver, ['mysql', 'mariadb'], true)) {
                $row = $pdo->query('SHOW REPLICA STATUS')->fetch(\PDO::FETCH_ASSOC);
                if (! $row) {
                    $row = $pdo->query('SHOW SLAVE STATUS')->fetch(\PDO::FETCH_ASSOC);
                }

                $behind = $row['Seconds_Behind_Source'] ?? $row['Seconds_Behind_Slave'] ?? null;

                return $behind === null ? [] : ['replication_lag_seconds' => (int) $behind];
            }
        } catch (\Throwable $e) {
            // Lag is best-effort; a monitoring user without the right grant simply
            // omits the metric.
        }

        return [];
    }
}
