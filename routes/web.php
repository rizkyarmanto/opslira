<?php

use App\Http\Controllers\Auth\AuthenticatedSessionController;
use Illuminate\Support\Facades\Route;

Route::get('/', function () {
    return redirect()->route('dashboard.noc');
})->name('home');

Route::middleware('guest')->group(function () {
    Route::get('/login', function () {
        return inertia('Auth/Login');
    })->name('login');

    Route::post('/login', [AuthenticatedSessionController::class, 'store']);
});

Route::post('/logout', [AuthenticatedSessionController::class, 'destroy'])
    ->middleware('auth')
    ->name('logout');

Route::get('/settings', function () {
    return inertia('Settings');
})->middleware('auth')->name('settings');
