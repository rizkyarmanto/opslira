<?php

use App\Models\User;
use App\Modules\Reporting\Services\SlaDashboardService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Support\Carbon;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\DB;

function ensureLogPartition(Carbon $date): void
{
    $suffix = $date->format('Y_m');
    $start = $date->copy()->startOfMonth()->format('Y-m-d H:i:sP');
    $end = $date->copy()->addMonthNoOverflow()->startOfMonth()->format('Y-m-d H:i:sP');

    DB::statement(
        "CREATE TABLE IF NOT EXISTS monitoring_logs_{$suffix} PARTITION OF monitoring_logs
         FOR VALUES FROM ('{$start}') TO ('{$end}')"
    );
}

function seedLogsForTarget(MonitoringTarget $target, Carbon $start, int $minutes, string $status = 'UP'): void
{
    ensureLogPartition($start);
    ensureLogPartition($start->copy()->addMinutes($minutes));

    $rows = [];
    for ($i = 0; $i < $minutes; $i++) {
        $rows[] = [
            'target_id' => $target->id,
            'status' => $status,
            'response_time_ms' => 100,
            'http_status_code' => 200,
            'error_message' => null,
            'metrics_payload' => '{}',
            'checked_at' => $start->copy()->addMinutes($i)->toDateTimeString(),
        ];
    }
    foreach (array_chunk($rows, 500) as $chunk) {
        DB::table('monitoring_logs')->insert($chunk);
    }
}

it('computes pooled monthly sla from logs', function () {
    $target = MonitoringTarget::factory()->create(['degraded_threshold_ms' => 5000]);
    $start = Carbon::parse('2026-09-01 00:00:00', 'Asia/Jakarta');

    seedLogsForTarget($target, $start, 60, 'UP');

    $svc = app(SlaDashboardService::class);
    $res = $svc->forWindow($start->copy(), $start->copy()->addHour(), [$target->id]);

    expect($res['has_data'])->toBeTrue()
        ->and($res['sla'])->toEqual(100.0)
        ->and($res['targets'])->toHaveCount(1);
});

it('measures downtime from down segments inside the month', function () {
    $target = MonitoringTarget::factory()->create(['degraded_threshold_ms' => 5000]);
    $monthStart = Carbon::parse('2026-09-01 00:00:00', 'Asia/Jakarta');

    // 30 min UP then one DOWN log owning 2h15m until the next UP log.
    seedLogsForTarget($target, $monthStart, 30, 'UP');
    DB::table('monitoring_logs')->insert([
        'target_id' => $target->id,
        'status' => 'DOWN',
        'response_time_ms' => 100,
        'http_status_code' => 200,
        'error_message' => 'timeout',
        'metrics_payload' => '{}',
        'checked_at' => $monthStart->copy()->addMinutes(30)->toDateTimeString(),
    ]);
    seedLogsForTarget($target, $monthStart->copy()->addMinutes(165), 30, 'UP');

    $svc = app(SlaDashboardService::class);
    $res = $svc->forWindow($monthStart->copy(), $monthStart->copy()->addHours(3), [$target->id]);

    expect($res['has_data'])->toBeTrue()
        ->and($res['downtime_seconds'])->toEqual(8100)
        ->and(round($res['sla'], 2))->toEqual(25.0);
});

it('returns null sla when a window has no logs', function () {
    $target = MonitoringTarget::factory()->create();
    $svc = app(SlaDashboardService::class);

    $res = $svc->forWindow(
        Carbon::parse('2026-09-01 00:00:00', 'Asia/Jakarta'),
        Carbon::parse('2026-09-02 00:00:00', 'Asia/Jakarta'),
        [$target->id]
    );

    expect($res['has_data'])->toBeFalse()
        ->and($res['sla'])->toBeNull();
});

it('renders the NOC overview with time-based sla props', function () {
    $user = User::factory()->create();
    $target = MonitoringTarget::factory()->create(['name' => 'SLA Wiring Probe', 'degraded_threshold_ms' => 5000]);
    $now = now()->setTimezone('Asia/Jakarta');
    seedLogsForTarget($target, $now->copy()->startOfDay(), 10, 'UP');

    // Pooled global SLA covers every target with logs, so the expectation is
    // computed live from the same service instead of a hardcoded 100.
    $expected = app(SlaDashboardService::class)->forWindow(
        $now->copy()->startOfMonth(),
        $now->copy()->endOfMonth()
    );
    $expectedGlobal = $expected['sla'] === null ? null : round($expected['sla'], 2);

    $this->actingAs($user)
        ->get('/dashboard/noc')
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Dashboard/NocOverview')
            ->where('slaWindowDays', $now->daysInMonth)
            ->where('globalSla', $expectedGlobal)
            ->has('slaTrend', 30)
            ->has('targetDailySla')
        );
});

it('hands the page a number when the cache holds the sla as a numeric string', function () {
    $user = User::factory()->create();
    MonitoringTarget::factory()->create(['degraded_threshold_ms' => 5000]);

    // Redis, the production cache store, keeps numeric values as plain strings
    // and returns them without a cast, so a cached 99.94 arrives as '99.94'.
    Cache::put('noc:sla:global', '99.94', 60);

    $this->actingAs($user)
        ->get('/dashboard/noc')
        ->assertOk()
        ->assertInertia(fn ($page) => $page
            ->component('Dashboard/NocOverview')
            ->where('globalSla', 99.94)
        );
});
