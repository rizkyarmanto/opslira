<?php

namespace App\Console\Commands;

use Illuminate\Console\Command;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\DB;

/**
 * Manual partition creation: fallback when the scheduler has not run yet.
 * Plan assumption: "command monitoring:create-partition {YYYY-MM}".
 */
class CreateMonitoringLogsPartition extends Command
{
    protected $signature = 'monitoring:create-partition {period : Format YYYY-MM}';

    protected $description = 'Create a monthly monitoring_logs partition manually';

    public function handle(): int
    {
        $period = $this->argument('period');

        if (! preg_match('/^\d{4}-\d{2}$/', $period)) {
            $this->error('Invalid period format. Use YYYY-MM.');

            return self::FAILURE;
        }

        $date = Carbon::createFromFormat('Y-m', $period)->startOfMonth();
        $suffix = $date->format('Y_m');
        $start = $date->copy()->startOfMonth()->format('Y-m-d H:i:sP');
        $end = $date->copy()->addMonthNoOverflow()->startOfMonth()->format('Y-m-d H:i:sP');

        DB::statement(
            "CREATE TABLE IF NOT EXISTS monitoring_logs_{$suffix} PARTITION OF monitoring_logs
             FOR VALUES FROM ('{$start}') TO ('{$end}')"
        );

        $this->info("Partition monitoring_logs_{$suffix} ensured.");

        return self::SUCCESS;
    }
}
