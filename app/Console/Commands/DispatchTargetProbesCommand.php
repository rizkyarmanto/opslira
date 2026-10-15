<?php

namespace App\Console\Commands;

use App\Modules\Probes\Jobs\ExecuteTargetProbeJob;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Console\Command;

/**
 * Dispatches probe jobs for targets due for a check (runs every minute).
 */
class DispatchTargetProbesCommand extends Command
{
    protected $signature = 'monitoring:dispatch-probes';

    protected $description = 'Dispatch probe jobs for targets whose next_check_at is due';

    public function handle(): int
    {
        $targets = MonitoringTarget::dueForCheck()->get();
        $dispatched = 0;

        foreach ($targets as $target) {
            ExecuteTargetProbeJob::dispatch($target->id);

            // Reschedule next check
            $target->update([
                'next_check_at' => now()->addSeconds($target->check_interval_seconds),
            ]);

            $dispatched++;
        }

        $this->info("Dispatched {$dispatched} probe job(s).");

        return self::SUCCESS;
    }
}
