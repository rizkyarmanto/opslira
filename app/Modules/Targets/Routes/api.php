<?php

use App\Modules\Targets\Controllers\TargetController;
use Illuminate\Support\Facades\Route;

Route::middleware(['web', 'auth'])->group(function () {
    Route::get('/targets', [TargetController::class, 'index'])->name('api.targets.index');
    Route::get('/targets/search', [TargetController::class, 'search'])->name('api.targets.search');
    Route::post('/targets', [TargetController::class, 'store'])->name('api.targets.store');
    Route::post('/targets/test-connection', [TargetController::class, 'testConnectionPayload'])->name('api.targets.test-connection.payload');
    Route::get('/targets/{target}', [TargetController::class, 'show'])->name('api.targets.show');
    Route::get('/targets/{target}/incidents', [TargetController::class, 'incidents'])->name('api.targets.incidents');
    Route::get('/targets/{target}/degraded-logs', [TargetController::class, 'degradedLogs'])->name('api.targets.degraded-logs');
    Route::put('/targets/{target}', [TargetController::class, 'update'])->name('api.targets.update');
    Route::delete('/targets/{target}', [TargetController::class, 'destroy'])->name('api.targets.destroy');
    Route::post('/targets/{target}/enable', [TargetController::class, 'enable'])->name('api.targets.enable');
});
