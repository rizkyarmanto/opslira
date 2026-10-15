<?php

namespace App\Console\Commands;

use App\Modules\Reporting\Jobs\GenerateAndSendReportJob;
use App\Modules\Reporting\Models\ReportSchedule;
use App\Modules\Reporting\Services\CronNextRunCalculator;
use Illuminate\Console\Command;

/**
 * Dispatches report generation for schedules whose next_run_at is due (every minute).
 */
class DispatchScheduledReportsCommand extends Command
{
    protected $signature = 'monitoring:dispatch-reports';

    protected $description = 'Dispatch report generation jobs for due schedules';

    public function handle(CronNextRunCalculator $calculator): int
    {
        $schedules = ReportSchedule::due()->get();
        $dispatched = 0;

        foreach ($schedules as $schedule) {
            GenerateAndSendReportJob::dispatch($schedule->id)->onQueue('reports');

            $schedule->update([
                'last_run_at' => now(),
                'next_run_at' => $calculator->next($schedule->cron_expression, $schedule->timezone),
            ]);

            $dispatched++;
        }

        $this->info("Dispatched {$dispatched} report job(s).");

        return self::SUCCESS;
    }
}
