<?php

use App\Modules\Reporting\Controllers\ReportRunnerController;
use Illuminate\Support\Facades\Route;

/*
 * Endpoint the Go runner calls to generate every due report schedule.
 *
 * Token only: no session, no CSRF.
 */
Route::middleware('runner')->group(function () {
    Route::post('/reports/dispatch', [ReportRunnerController::class, 'dispatch']);
});
