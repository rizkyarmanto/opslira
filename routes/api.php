<?php

use Illuminate\Support\Facades\Route;

// API routes are registered per-module in bootstrap/app.php.
// This file exists as the base api entrypoint.
Route::get('/health', fn () => response()->json(['status' => 'ok']));
