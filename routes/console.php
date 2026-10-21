<?php

/*
|--------------------------------------------------------------------------
| Scheduled Commands (Opslira)
|--------------------------------------------------------------------------
|
| There are no Schedule:: entries here on purpose. The Go runner (go/) owns
| every periodic trigger: it claims due targets, runs probes through the PHP
| adapters, dispatches due reports, and calls the rollup and prune commands
| through the internal endpoints. Leaving schedules here would double every
| probe and report.
|
| All four commands stay callable by hand:
|   php artisan monitoring:dispatch-probes
|   php artisan monitoring:rollup
|   php artisan monitoring:dispatch-reports
|   php artisan monitoring:prune-partitions
|
*/
