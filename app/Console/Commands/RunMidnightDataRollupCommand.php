<?php

namespace App\Console\Commands;

use App\Modules\Incidents\Models\Incident;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Console\Command;
use Illuminate\Support\Facades\DB;

/**
 * Aggregates yesterday's monitoring_logs into monitoring_daily_summaries.
 * Idempotent upsert on (target_id, summary_date).
 * docs/gemini/dokumen_arsitektur_aplikasi_web.md:136
 */
class RunMidnightDataRollupCommand extends Command
{
    protected $signature = 'monitoring:rollup {--date= : Specific date (Y-m-d), default yesterday}';

    protected $description = 'Roll up daily monitoring logs into summaries';

    public function handle(): int
    {
        $date = $this->option('date')
            ? \Illuminate\Support\Carbon::parse($this->option('date'))
            : now()->subDay();

        $start = $date->copy()->startOfDay();
        $end = $date->copy()->endOfDay();

        $targets = MonitoringTarget::all();
        $processed = 0;

        foreach ($targets as $target) {
            $stats = DB::table('monitoring_logs')
                ->where('target_id', $target->id)
                ->whereBetween('checked_at', [$start, $end])
                ->selectRaw('
                    COUNT(*) as total_checks,
                    COUNT(*) FILTER (WHERE status IN (\'UP\',\'DEGRADED\')) as successful_checks,
                    COUNT(*) FILTER (WHERE status = \'DOWN\') as failed_checks,
                    COALESCE(AVG(response_time_ms), 0) as avg_response_time_ms,
                    COALESCE(MAX(response_time_ms), 0) as max_response_time_ms,
                    COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time_ms), 0) as p95_response_time_ms
                ')
                ->first();

            if (! $stats || $stats->total_checks === 0) {
                continue;
            }

            $uptime = $stats->total_checks > 0
                ? round(($stats->successful_checks / $stats->total_checks) * 100, 4)
                : 100.0;

            $incidents = Incident::where('target_id', $target->id)
                ->whereBetween('started_at', [$start, $end])
                ->get();

            $downtimeSeconds = $incidents->sum(fn ($i) => $i->duration_seconds ?? 0);

            DB::table('monitoring_daily_summaries')->upsert(
                [[
                    'target_id' => $target->id,
                    'summary_date' => $date->toDateString(),
                    'total_checks' => $stats->total_checks,
                    'successful_checks' => $stats->successful_checks,
                    'failed_checks' => $stats->failed_checks,
                    'uptime_percentage' => $uptime,
                    'avg_response_time_ms' => round((float) $stats->avg_response_time_ms, 2),
                    'max_response_time_ms' => round((float) $stats->max_response_time_ms, 2),
                    'p95_response_time_ms' => round((float) $stats->p95_response_time_ms, 2),
                    'total_incidents' => $incidents->count(),
                    'total_downtime_seconds' => $downtimeSeconds,
                    'maintenance_seconds' => 0,
                    'daily_metrics_snapshot' => json_encode([]),
                    'created_at' => now(),
                ]],
                ['target_id', 'summary_date'],
                ['total_checks', 'successful_checks', 'failed_checks', 'uptime_percentage',
                    'avg_response_time_ms', 'max_response_time_ms', 'p95_response_time_ms',
                    'total_incidents', 'total_downtime_seconds', 'daily_metrics_snapshot'],
            );

            $processed++;
        }

        $this->info("Rolled up {$processed} target summary/ies for {$date->toDateString()}.");

        return self::SUCCESS;
    }
}
