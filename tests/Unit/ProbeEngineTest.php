<?php

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeAdapterFactory;
use App\Modules\Reporting\Services\SlaCalculatorService;

it('returns the correct adapter for every supported protocol', function () {
    $factory = new ProbeAdapterFactory();

    foreach ($factory->supportedProtocols() as $protocol) {
        $adapter = $factory->make($protocol);
        expect($adapter->supportsProtocol($protocol))->toBeTrue();
    }
});

it('throws for an unsupported protocol', function () {
    $factory = new ProbeAdapterFactory();
    $factory->make('UNKNOWN_PROTOCOL');
})->throws(InvalidArgumentException::class);

it('classifies probe result factories correctly', function () {
    expect(ProbeResult::up(120.0)->isUp())->toBeTrue();
    expect(ProbeResult::down('timeout')->isDown())->toBeTrue();
    expect(ProbeResult::degraded(1500.0, [], 'slow')->status)->toBe('DEGRADED');
});

it('exposes a concrete SLA calculator service', function () {
    expect(app(SlaCalculatorService::class))->toBeInstanceOf(SlaCalculatorService::class);
});
