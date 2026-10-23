<?php

use App\Modules\Reporting\Services\SlaPeriodService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Carbon;

// Shared helpers. No DB used anywhere in this file.
beforeEach(function () {
    $this->svc = new SlaPeriodService;
    $this->target = new MonitoringTarget([
        'name' => 't',
        'degraded_threshold_ms' => 5000,
    ]);
    // Interval helper: [$start, $end] strings parsed in given tz.
    $this->iv = fn ($s, $e, $tz = 'Asia/Jakarta') => [
        'start' => Carbon::parse($s, $tz),
        'end' => Carbon::parse($e, $tz),
    ];
});

it('reports 100 sla for a clean 30-day month', function () {
    $res = $this->svc->monthly($this->target, Carbon::parse('2026-04-15', 'Asia/Jakarta'));

    expect($res['total_seconds'])->toEqual(2592000)
        ->and($res['sla'])->toEqual(100.0)
        ->and($res['has_data'])->toBeTrue()
        ->and($res['month'])->toEqual('2026-04');
});

it('computes 99.6875 for 2h15m downtime in a 30-day window', function () {
    $start = Carbon::parse('2026-04-01 00:00:00', 'Asia/Jakarta');
    $end = Carbon::parse('2026-05-01 00:00:00', 'Asia/Jakarta');
    $down = [($this->iv)('2026-04-10 00:00:00', '2026-04-10 02:15:00')];

    $res = $this->svc->downtimeForWindow($this->target, $start, $end, [], $down);

    expect($res['total_seconds'])->toEqual(2592000)
        ->and($res['downtime_seconds'])->toEqual(8100)
        ->and($res['sla'])->toEqual(99.6875);
});

it('models 20 failed checks of 1440 as 98.6111 for one day', function () {
    // 20 failed 1-minute checks = 1200s down in an 86400s day.
    $down = [($this->iv)('2026-01-05 00:00:00', '2026-01-05 00:20:00')];

    $res = $this->svc->daily($this->target, Carbon::parse('2026-01-05', 'Asia/Jakarta'), [], $down);

    expect($res['total_seconds'])->toEqual(86400)
        ->and($res['downtime_seconds'])->toEqual(1200)
        ->and($res['sla'])->toEqual(98.6111);
});

it('uses 2419200s denominator for a 28-day february', function () {
    $res = $this->svc->monthly($this->target, Carbon::parse('2025-02-10', 'Asia/Jakarta'));

    expect($res['total_seconds'])->toEqual(2419200)
        ->and($res['sla'])->toEqual(100.0)
        ->and($res['month'])->toEqual('2025-02');
});

it('uses 2505600s denominator for leap february 2024', function () {
    $res = $this->svc->monthly($this->target, Carbon::parse('2024-02-10', 'Asia/Jakarta'));

    expect($res['total_seconds'])->toEqual(2505600)
        ->and($res['sla'])->toEqual(100.0);
});

it('attributes a 23:30Z incident to the next jakarta day', function () {
    // 2026-01-01 23:30Z = 2026-01-02 06:30+07.
    $down = [[
        'start' => Carbon::parse('2026-01-01 23:30:00', 'UTC'),
        'end' => Carbon::parse('2026-01-01 23:45:00', 'UTC'),
    ]];

    $day1 = $this->svc->daily($this->target, Carbon::parse('2026-01-01', 'Asia/Jakarta'), [], $down);
    $day2 = $this->svc->daily($this->target, Carbon::parse('2026-01-02', 'Asia/Jakarta'), [], $down);

    expect($day1['downtime_seconds'])->toEqual(0)
        ->and($day2['downtime_seconds'])->toEqual(900);
});

it('returns null sla with has_data false for an empty window', function () {
    $start = Carbon::parse('2026-04-01 00:00:00', 'Asia/Jakarta');
    $end = Carbon::parse('2026-04-02 00:00:00', 'Asia/Jakarta');

    $res = $this->svc->downtimeForWindow($this->target, $start, $end, [], [], false);

    expect($res['has_data'])->toBeFalse()
        ->and($res['sla'])->toBeNull();

    $daily = $this->svc->daily($this->target, Carbon::parse('2026-04-01', 'Asia/Jakarta'), [], [], false);

    expect($daily['has_data'])->toBeFalse()
        ->and($daily['sla'])->toBeNull();
});

it('splits maintenance crossing midnight by clipping to each day', function () {
    // Maint 23:00 day1 to 01:00 day2; down 22:30-23:30 day1 (half overlapped).
    $maint = [($this->iv)('2026-03-10 23:00:00', '2026-03-11 01:00:00')];
    $down = [($this->iv)('2026-03-10 22:30:00', '2026-03-10 23:30:00')];

    $res = $this->svc->daily($this->target, Carbon::parse('2026-03-10', 'Asia/Jakarta'), $maint, $down);

    expect($res['maintenance_seconds'])->toEqual(3600)
        ->and($res['downtime_seconds'])->toEqual(1800);
});

it('excludes downtime fully inside maintenance without double subtracting', function () {
    // Down 2h fully inside 4h maintenance.
    $maint = [($this->iv)('2026-03-11 10:00:00', '2026-03-11 14:00:00')];
    $down = [($this->iv)('2026-03-11 11:00:00', '2026-03-11 13:00:00')];

    $res = $this->svc->daily($this->target, Carbon::parse('2026-03-11', 'Asia/Jakarta'), $maint, $down);

    expect($res['maintenance_seconds'])->toEqual(14400)
        ->and($res['downtime_seconds'])->toEqual(0)
        ->and($res['sla'])->toEqual(100.0);
});

it('merges overlapping down segments passed out of order', function () {
    // Three DOWN segments out of order, two overlapping; merged must equal 4h.
    $down = [
        ($this->iv)('2026-03-12 12:00:00', '2026-03-12 13:00:00'),
        ($this->iv)('2026-03-12 09:00:00', '2026-03-12 11:00:00'),
        ($this->iv)('2026-03-12 10:30:00', '2026-03-12 12:30:00'),
    ];

    $res = $this->svc->daily($this->target, Carbon::parse('2026-03-12', 'Asia/Jakarta'), [], $down);

    expect($res['downtime_seconds'])->toEqual(14400);
});

it('computes the error budget table per day and per 30-day month', function () {
    // Allowed = (1 - target) * period. Day = 86400s, month30 = 2592000s.
    // Tolerance compare: (1 - target) leaves a binary float artifact.
    expect(abs($this->svc->errorBudget(0.99, 86400)['allowed_seconds'] - 864.0))->toBeLessThan(0.0001)
        ->and(abs($this->svc->errorBudget(0.999, 86400)['allowed_seconds'] - 86.4))->toBeLessThan(0.0001)
        ->and(abs($this->svc->errorBudget(0.9995, 86400)['allowed_seconds'] - 43.2))->toBeLessThan(0.0001)
        ->and(abs($this->svc->errorBudget(0.9999, 86400)['allowed_seconds'] - 8.64))->toBeLessThan(0.0001);

    expect(abs($this->svc->errorBudget(0.99, 2592000)['allowed_seconds'] - 25920.0))->toBeLessThan(0.0001)
        ->and(abs($this->svc->errorBudget(0.999, 2592000)['allowed_seconds'] - 2592.0))->toBeLessThan(0.0001)
        ->and(abs($this->svc->errorBudget(0.9995, 2592000)['allowed_seconds'] - 1296.0))->toBeLessThan(0.0001)
        ->and(abs($this->svc->errorBudget(0.9999, 2592000)['allowed_seconds'] - 259.2))->toBeLessThan(0.0001);

    // Shape check on one row.
    $row = $this->svc->errorBudget(0.99, 86400);
    expect($row)->toHaveKeys(['allowed_seconds', 'allowed_minutes', 'allowed_minutes_seconds']);
});

it('classifies down logs by status code and latency threshold', function () {
    expect($this->svc->isDown(['status' => 'DOWN', 'http_status_code' => 200, 'response_time_ms' => 100], $this->target))->toBeTrue()
        ->and($this->svc->isDown(['status' => 'UP', 'http_status_code' => 503, 'response_time_ms' => 100], $this->target))->toBeTrue()
        ->and($this->svc->isDown(['status' => 'UP', 'http_status_code' => 200, 'response_time_ms' => 6000], $this->target))->toBeTrue()
        ->and($this->svc->isDown(['status' => 'UP', 'http_status_code' => 200, 'response_time_ms' => 100], $this->target))->toBeFalse()
        ->and($this->svc->isDown(['status' => 'UP', 'http_status_code' => 200, 'response_time_ms' => 5000], $this->target))->toBeFalse();
});

it('reports status against a fractional target with remaining budget', function () {
    // Target 0.99 means threshold 99.0.
    $met = $this->svc->status(99.9, 0.99, 100.0, 864.0);
    $missed = $this->svc->status(98.0, 0.99, 1000.0, 864.0);

    expect($met['met'])->toBeTrue()
        ->and($met['remaining_seconds'])->toEqual(764.0)
        ->and($met)->toHaveKeys(['met', 'remaining_seconds', 'remaining_human']);

    expect($missed['met'])->toBeFalse()
        ->and($missed['remaining_seconds'])->toEqual(-136.0);
});
