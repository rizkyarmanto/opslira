<?php

namespace App\Modules\Reporting\Services;

use App\Modules\Reporting\Jobs\GenerateAndSendReportJob;
use App\Modules\Reporting\Models\ReportSchedule;

/**
 * Generates every report schedule whose next_run_at has passed.
 *
 * Jobs are dispatched synchronously: no PHP queue worker runs any more, so a
 * queued job would sit in Redis forever. Inline execution also lets a failure
 * propagate to the caller, which leaves the schedule due so the next runner
 * tick retries it.
 */
class ReportDispatchService
{
    public function __construct(private readonly CronNextRunCalculator $calculator)
    {
    }

    public function dispatchDue(): int
    {
        $dispatched = 0;

        foreach (ReportSchedule::due()->get() as $schedule) {
            GenerateAndSendReportJob::dispatchSync($schedule->id);

            $schedule->update([
                'last_run_at' => now(),
                'next_run_at' => $this->calculator->next($schedule->cron_expression, $schedule->timezone),
            ]);

            $dispatched++;
        }

        return $dispatched;
    }
}
