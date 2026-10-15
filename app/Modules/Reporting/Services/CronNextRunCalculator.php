<?php

namespace App\Modules\Reporting\Services;

use Cron\CronExpression;
use Illuminate\Support\Carbon;

/**
 * Computes the next run time for a cron expression in a given timezone.
 */
class CronNextRunCalculator
{
    public function next(string $cronExpression, string $timezone = 'Asia/Jakarta'): Carbon
    {
        $cron = new CronExpression($cronExpression);

        $now = Carbon::now($timezone);
        $nextRun = $cron->getNextRunDate($now->toDateTimeString(), 0, false, $timezone);

        return Carbon::instance($nextRun)->setTimezone(config('app.timezone'));
    }
}
