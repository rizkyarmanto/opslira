<?php

use App\Modules\Incidents\Controllers\IncidentController;
use Illuminate\Support\Facades\Route;

Route::middleware(['web', 'auth'])->group(function () {
    Route::post('/incidents/{incident}/resolve', [IncidentController::class, 'resolve'])->name('api.incidents.resolve');
});
