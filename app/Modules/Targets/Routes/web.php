<?php

use App\Modules\Targets\Controllers\TargetController;
use Illuminate\Support\Facades\Route;

Route::middleware(['web', 'auth'])->prefix('targets')->name('targets.')->group(function () {
    Route::get('/', [TargetController::class, 'index'])->name('index');
    Route::post('/', [TargetController::class, 'store'])->name('store');
    Route::post('/test-connection', [TargetController::class, 'testConnectionPayload'])->name('test-connection.payload');
    Route::get('/{target}', [TargetController::class, 'show'])->name('show');
    Route::put('/{target}', [TargetController::class, 'update'])->name('update');
    Route::delete('/{target}', [TargetController::class, 'destroy'])->name('destroy');
    Route::post('/{target}/enable', [TargetController::class, 'enable'])->name('enable');
});
