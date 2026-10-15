<?php

use App\Modules\Reporting\Controllers\ReportController;
use Illuminate\Support\Facades\Route;

Route::middleware(['web', 'auth'])->prefix('reports')->name('reports.')->group(function () {
    Route::get('/schedules', [ReportController::class, 'schedules'])->name('schedules');
    Route::post('/schedules', [ReportController::class, 'store'])->name('schedules.store');
    Route::put('/schedules/{schedule}', [ReportController::class, 'update'])->name('schedules.update');
    Route::delete('/schedules/{schedule}', [ReportController::class, 'destroy'])->name('schedules.destroy');
    Route::post('/schedules/{schedule}/run-now', [ReportController::class, 'runNow'])->name('schedules.run-now');
    Route::get('/email-logs', [ReportController::class, 'emailLogs'])->name('email-logs');
    Route::get('/download/{report}/{format}', [ReportController::class, 'download'])->name('download');
});
