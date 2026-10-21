<?php

use App\Modules\Probes\Contracts\ProbeResult;
use App\Modules\Probes\Services\ProbeFailureLogger;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Facades\Log;

function failureTarget(): MonitoringTarget
{
    return new MonitoringTarget([
        'name' => 'Probe login',
        'dashboard_group' => 'NETWORK',
        'target_type' => 'INTERNET',
        'host_or_endpoint' => 'https://user:pass@monitoring.example.id',
        'protocol_type' => 'ICMP_PING',
    ]);
}

it('masks credential shaped values while keeping the labels', function () {
    $masked = ProbeFailureLogger::mask('password=hunter2 token=abc Authorization: Bearer x');

    expect($masked)
        ->not->toContain('hunter2')
        ->not->toContain('abc')
        ->not->toContain(' x')
        ->toContain('password=')
        ->toContain('token=')
        ->toContain('Authorization:');
});

it('masks user info embedded in a url and truncates a long message', function () {
    $masked = ProbeFailureLogger::mask('failed to reach ldap://monitor:supersecret@ldap.example.id/api');

    expect($masked)->not->toContain('supersecret')->toContain('ldap://monitor:****@ldap.example.id/api');

    $long = ProbeFailureLogger::mask(str_repeat('body', 900));

    expect(strlen($long))->toBe(1000);
});

it('logs a warning for a non-up result with the target identity and no secret', function () {
    Log::shouldReceive('warning')->once()->withArgs(function (string $message, array $context) {
        return $message === 'Target probe failed'
            && $context['target_type'] === 'INTERNET'
            && $context['protocol_type'] === 'ICMP_PING'
            && $context['phase'] === 'scheduled'
            && $context['status'] === 'DOWN'
            && $context['error_message'] === 'auth failed: password=****'
            && str_contains((string) $context['host'], '****@');
    });

    $result = ProbeResult::down('auth failed: password=hunter2');
    (new ProbeFailureLogger())->record(failureTarget(), $result, 'scheduled');
});

it('stays silent for an up result', function () {
    Log::shouldReceive('warning')->never();

    (new ProbeFailureLogger())->record(failureTarget(), ProbeResult::up(12.0, ['packet_loss_pct' => 0]), 'scheduled');
});
