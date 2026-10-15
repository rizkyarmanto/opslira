<?php

use App\Modules\Dashboard\Controllers\DashboardController;
use Illuminate\Support\Facades\Route;

// Lightweight polling endpoint, Redis-only reads (docs/gemini/dokumen_arsitektur_aplikasi_web.md:97)
Route::middleware(['web', 'auth'])->group(function () {
    Route::get('/dashboard/states', [DashboardController::class, 'states'])->name('api.dashboard.states');
});
