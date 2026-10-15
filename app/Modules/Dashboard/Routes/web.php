<?php

use App\Modules\Dashboard\Controllers\DashboardController;
use Illuminate\Support\Facades\Route;

Route::middleware(['web', 'auth'])->prefix('dashboard')->name('dashboard.')->group(function () {
    Route::get('/noc', [DashboardController::class, 'noc'])->name('noc');
    Route::get('/network', [DashboardController::class, 'network'])->name('network');
    Route::get('/apps-domains-db', [DashboardController::class, 'appsDomainsDb'])->name('apps-domains-db');
    Route::get('/mail-s3-beszel', [DashboardController::class, 'mailS3Beszel'])->name('mail-s3-beszel');
});
