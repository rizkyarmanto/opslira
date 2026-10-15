<?php

use App\Modules\Reporting\Controllers\ReportController;
use Illuminate\Support\Facades\Route;

/*
 * JSON endpoints for the reports screens. The Inertia page routes live in web.php;
 * these are the mutations the frontend calls through /api/v1.
 */
Route::middleware(['web', 'auth'])->group(function () {
    Route::post('/reports/schedules', [ReportController::class, 'store'])->name('api.reports.schedules.store');
    Route::put('/reports/schedules/{schedule}', [ReportController::class, 'update'])->name('api.reports.schedules.update');
    Route::delete('/reports/schedules/{schedule}', [ReportController::class, 'destroy'])->name('api.reports.schedules.destroy');
    Route::post('/reports/schedules/{schedule}/run-now', [ReportController::class, 'runNow'])->name('api.reports.schedules.run-now');
});
