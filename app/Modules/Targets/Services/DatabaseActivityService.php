<?php

namespace App\Modules\Targets\Services;

use App\Modules\Targets\Models\MonitoringTarget;
use PDO;
use RuntimeException;

class DatabaseActivityService
{
    public function __construct(private readonly CredentialEncryptionService $encryption) {}

    public function fetch(MonitoringTarget $target): array
    {
        $config = $target->config_payload ?? [];
        $driver = (string) ($config['db_driver'] ?? 'pgsql');
        $host = $target->host_or_endpoint;
        $database = (string) ($config['database_name'] ?? '');
        $port = (int) ($target->port ?: match ($driver) {
            'mysql', 'mariadb' => 3306,
            'sqlsrv' => 1433,
            default => 5432,
        });
        $dsn = match ($driver) {
            'pgsql' => "pgsql:host={$host};port={$port};dbname={$database}",
            'mysql', 'mariadb' => "mysql:host={$host};port={$port};dbname={$database}",
            'sqlsrv' => "sqlsrv:Server={$host},{$port};Database={$database}",
            default => throw new RuntimeException('Unsupported database driver'),
        };
        $credentials = [];
        if ($credential = $target->credentials) {
            $credentials = $this->encryption->decrypt(
                $credential->encrypted_payload,
                $credential->encryption_iv,
                $credential->encryption_tag,
            );
        }

        $sql = match ($driver) {
            'pgsql' => <<<'SQL'
                SELECT usename AS username, datname AS database, state,
                    EXTRACT(EPOCH FROM (NOW() - state_change)) AS state_seconds,
                    EXTRACT(EPOCH FROM (NOW() - query_start)) AS query_seconds,
                    query AS query_syntax
                FROM pg_stat_activity
                WHERE backend_type = 'client backend'
                ORDER BY state_change ASC NULLS LAST
                SQL,
            'mysql', 'mariadb' => <<<'SQL'
                SELECT USER AS username, DB AS `database`, COMMAND AS state, TIME AS state_seconds,
                    CASE WHEN COMMAND = 'Query' THEN TIME ELSE NULL END AS query_seconds,
                    INFO AS query_syntax
                FROM information_schema.PROCESSLIST
                ORDER BY TIME DESC
                SQL,
            'sqlsrv' => <<<'SQL'
                SELECT s.login_name AS username, DB_NAME(COALESCE(r.database_id, s.database_id)) AS [database],
                    COALESCE(r.status, s.status) AS state,
                    DATEDIFF(SECOND, COALESCE(r.start_time, s.last_request_end_time, s.login_time), GETDATE()) AS state_seconds,
                    CASE WHEN r.start_time IS NOT NULL THEN DATEDIFF(SECOND, r.start_time, GETDATE()) ELSE NULL END AS query_seconds,
                    COALESCE(active_sql.text, last_sql.text) AS query_syntax
                FROM sys.dm_exec_sessions s
                LEFT JOIN sys.dm_exec_requests r ON s.session_id = r.session_id
                LEFT JOIN sys.dm_exec_connections c ON s.session_id = c.session_id
                OUTER APPLY sys.dm_exec_sql_text(r.sql_handle) active_sql
                OUTER APPLY sys.dm_exec_sql_text(c.most_recent_sql_handle) last_sql
                WHERE s.is_user_process = 1
                ORDER BY state_seconds DESC
                SQL,
        };

        $pdo = new PDO($dsn, (string) ($credentials['username'] ?? ''), (string) ($credentials['password'] ?? ''), [
            PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
            PDO::ATTR_TIMEOUT => $target->timeout_seconds ?? 5,
        ]);
        try {
            $statement = $pdo->query($sql);
            $rows = $statement->fetchAll(PDO::FETCH_ASSOC);
        } finally {
            $statement = null;
            $pdo = null;
        }

        return array_map(static fn (array $row): array => [
            'username' => $row['username'] === null ? null : (string) $row['username'],
            'database' => $row['database'] === null ? null : (string) $row['database'],
            'state' => $row['state'] === null ? null : (string) $row['state'],
            'state_duration' => self::formatDuration($row['state_seconds']),
            'query_age' => self::formatDuration($row['query_seconds']),
            'query_syntax' => $row['query_syntax'] === null ? null : (string) $row['query_syntax'],
        ], $rows);
    }

    private static function formatDuration(mixed $seconds): ?string
    {
        if ($seconds === null) {
            return null;
        }

        $seconds = max(0, (int) $seconds);

        return sprintf('%dd %02dh %02dm %02ds', intdiv($seconds, 86400), intdiv($seconds % 86400, 3600), intdiv($seconds % 3600, 60), $seconds % 60);
    }
}
