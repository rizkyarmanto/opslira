<?php

namespace App\Console\Commands;

use App\Modules\Reporting\Services\ReportDispatchService;
use Illuminate\Console\Command;

/**
 * Dispatches report generation for schedules whose next_run_at is due.
 *
 * The Go runner triggers this work through the internal endpoint; the command
 * stays so an operator can run the same path by hand.
 */
class DispatchScheduledReportsCommand extends Command
{
    protected $signature = 'monitoring:dispatch-reports';

    protected $description = 'Dispatch report generation jobs for due schedules';

    public function handle(ReportDispatchService $service): int
    {
        $dispatched = $service->dispatchDue();

        $this->info("Dispatched {$dispatched} report job(s).");

        return self::SUCCESS;
    }
}
