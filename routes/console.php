<?php

use Illuminate\Support\Facades\Schedule;

/*
|--------------------------------------------------------------------------
| Scheduled Commands (Opslira)
|--------------------------------------------------------------------------
| Master Scheduler: docs/gemini/dokumen_arsitektur_aplikasi_web.md:227-230
*/

// Probe dispatch: every minute (targets due for check)
Schedule::command('monitoring:dispatch-probes')
    ->everyMinute()
    ->withoutOverlapping()
    ->runInBackground();

// Midnight rollup: aggregate yesterday's logs into daily summaries (00:05 WIB)
Schedule::command('monitoring:rollup')
    ->dailyAt('00:05')
    ->timezone('Asia/Jakarta')
    ->withoutOverlapping();

// Scheduled report dispatch: every minute (schedules due for run)
Schedule::command('monitoring:dispatch-reports')
    ->everyMinute()
    ->withoutOverlapping()
    ->runInBackground();

// Partition maintenance: prune > 90 days + create next 2 months (02:00 WIB)
Schedule::command('monitoring:prune-partitions')
    ->dailyAt('02:00')
    ->timezone('Asia/Jakarta')
    ->withoutOverlapping();
