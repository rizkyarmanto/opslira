<?php

namespace App\Providers;

use App\Modules\Probes\Contracts\ProbeAdapterInterface;
use App\Modules\Probes\Services\ProbeAdapterFactory;
use Illuminate\Support\ServiceProvider;

class AppServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        // Probe engine: factory resolves adapter per protocol_type
        $this->app->singleton(ProbeAdapterFactory::class, function () {
            return new ProbeAdapterFactory();
        });
    }

    public function boot(): void
    {
        //
    }
}
