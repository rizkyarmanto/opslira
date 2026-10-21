<?php

use App\Modules\Probes\Controllers\RunnerController;
use Illuminate\Support\Facades\Route;

/*
 * Endpoints the Go runner calls for probe scheduling and maintenance.
 *
 * Token only: no session, no CSRF. Registered under the /internal prefix with
 * the `runner` middleware and nothing else.
 */
Route::middleware('runner')->group(function () {
    Route::post('/probes/claim', [RunnerController::class, 'claim']);
    Route::post('/probes/{target}', [RunnerController::class, 'probe']);
    Route::post('/maintenance/rollup', [RunnerController::class, 'rollup']);
    Route::post('/maintenance/prune', [RunnerController::class, 'prune']);
});
