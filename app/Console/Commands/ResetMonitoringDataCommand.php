<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Facades\DB;
use Illuminate\Support\Facades\Redis;

/**
 * Operator-run reset: wipes monitoring, incident and report data and reseeds
 * once, so a demo or handover environment starts from known rows. There is no
 * undo, which is why the confirmation prompt and the --force flag both exist.
 */
class ResetMonitoringDataCommand extends Command
{
    protected $signature = 'monitoring:reset-data {--force : Skip the confirmation prompt}';

    protected $description = 'Wipe monitoring, incident and report data, then seed once';

    public function handle(): int
    {
        if (! $this->option('force') && ! $this->confirm('This deletes all targets, credentials, logs, incidents and report history. Continue?')) {
            $this->warn('Aborted. Nothing was deleted.');

            return self::FAILURE;
        }

        // monitoring_logs is range-partitioned; PostgreSQL truncates the partitions
        // along with the parent when the parent is named in the statement.
        DB::statement(
            'TRUNCATE TABLE email_delivery_logs, generated_reports, report_schedules, incidents, monitoring_daily_summaries, target_credentials, monitoring_targets, monitoring_logs RESTART IDENTITY CASCADE'
        );

        $flushed = $this->flushProbeState();
        $this->info("Flushed {$flushed} probe state key(s).");

        $this->call('db:seed', ['--force' => true]);

        $counts = [
            'targets' => DB::table('monitoring_targets')->count(),
            'logs' => DB::table('monitoring_logs')->count(),
            'incidents' => DB::table('incidents')->count(),
            'users' => DB::table('users')->count(),
        ];

        foreach ($counts as $label => $count) {
            $this->info("{$label}: {$count}");
        }

        return self::SUCCESS;
    }

    /**
     * Drops the cached probe state and failure counters so no stale status from
     * a deleted target survives the reset. KEYS is safe here: this runs from an
     * operator command, never from a request path.
     */
    private function flushProbeState(): int
    {
        $redis = Redis::connection();
        $keys = [
            ...$redis->keys('target:state:*'),
            ...$redis->keys('target:failures:*'),
        ];

        foreach (array_chunk($keys, 500) as $chunk) {
            $redis->del($chunk);
        }

        return count($keys);
    }
}
