<?php

use App\Http\Middleware\EnsureAccountIsActive;
use App\Http\Middleware\EnsureRunnerToken;
use App\Http\Middleware\EnsureSuperAdmin;
use App\Http\Middleware\HandleInertiaRequests;
use Illuminate\Foundation\Application;
use Illuminate\Foundation\Configuration\Exceptions;
use Illuminate\Foundation\Configuration\Middleware;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Route;

return Application::configure(basePath: dirname(__DIR__))
    ->withRouting(
        web: __DIR__.'/../routes/web.php',
        api: __DIR__.'/../routes/api.php',
        commands: __DIR__.'/../routes/console.php',
        health: '/up',
        then: function () {
            // Modular routes: each module route file declares its own
            // middleware stack (web + auth).
            Route::group([], base_path('app/Modules/Dashboard/Routes/web.php'));
            Route::group([], base_path('app/Modules/Targets/Routes/web.php'));
            Route::group([], base_path('app/Modules/Reporting/Routes/web.php'));
            Route::group([], base_path('app/Modules/Users/Routes/web.php'));
            Route::prefix('api/v1')
                ->group(base_path('app/Modules/Targets/Routes/api.php'));
            Route::prefix('api/v1')
                ->group(base_path('app/Modules/Incidents/Routes/api.php'));
            Route::prefix('api/v1')
                ->group(base_path('app/Modules/Dashboard/Routes/api.php'));
            Route::prefix('api/v1')
                ->group(base_path('app/Modules/Reporting/Routes/api.php'));
            Route::prefix('api/v1')
                ->group(base_path('app/Modules/Users/Routes/api.php'));

            // Internal runner endpoints: token only, never the web stack.
            Route::prefix('internal')
                ->group(base_path('app/Modules/Probes/Routes/internal.php'));
            Route::prefix('internal')
                ->group(base_path('app/Modules/Reporting/Routes/internal.php'));
        },
    )
    ->withMiddleware(function (Middleware $middleware) {
        // App HTTP di belakang HAProxy HTTPS: percaya X-Forwarded-Proto
        // agar $request->isSecure() true dan @vite hasilkan URL https.
        $middleware->trustProxies(at: '*');
        $middleware->alias([
            'super_admin' => EnsureSuperAdmin::class,
            'runner' => EnsureRunnerToken::class,
        ]);

        $middleware->web(append: [
            EnsureAccountIsActive::class,
            HandleInertiaRequests::class,
        ]);
    })
    ->withExceptions(function (Exceptions $exceptions) {
        //
    })->create();
