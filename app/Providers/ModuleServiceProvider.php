<?php

namespace App\Providers;

use App\Console\Commands\CreateMonitoringLogsPartition;
use App\Console\Commands\DispatchScheduledReportsCommand;
use App\Console\Commands\DispatchTargetProbesCommand;
use App\Console\Commands\PruneOldMonitoringPartitions;
use App\Console\Commands\RunMidnightDataRollupCommand;
use Illuminate\Support\ServiceProvider;

class ModuleServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        //
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->commands([
                DispatchTargetProbesCommand::class,
                RunMidnightDataRollupCommand::class,
                DispatchScheduledReportsCommand::class,
                PruneOldMonitoringPartitions::class,
                CreateMonitoringLogsPartition::class,
            ]);
        }
    }
}
