<?php

use App\Modules\Users\Controllers\UserController;
use Illuminate\Support\Facades\Route;

Route::middleware(['web', 'auth', 'super_admin'])->group(function () {
    Route::post('/users', [UserController::class, 'store'])->name('api.users.store');
    Route::put('/users/{user}', [UserController::class, 'update'])->name('api.users.update');
    Route::delete('/users/{user}', [UserController::class, 'destroy'])->name('api.users.destroy');
});
