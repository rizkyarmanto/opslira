<?php

namespace App\Modules\Dashboard\Controllers;

use App\Modules\Incidents\Models\Incident;
use App\Modules\Probes\Services\TargetStateService;
use App\Modules\Reporting\Services\SlaDashboardService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\Cache;
use Illuminate\Support\Facades\DB;
use Inertia\Inertia;
use Inertia\Response;

class DashboardController
{
    /** Bucket width of the 24 h traffic chart. */
    private const TRAFFIC_BUCKET_MINUTES = 5;

    /** Devices drawn on the traffic chart before the list is shortened. */
    private const TRAFFIC_DEVICE_LIMIT = 6;

    public function __construct(private readonly SlaDashboardService $sla, private readonly TargetStateService $state)
    {
    }

    /**
     * Executive NOC Overview: all targets grouped, global SLA.
     */
    public function noc(): Response
    {
        $targets = MonitoringTarget::where('is_active', true)->get(['id', 'name', 'dashboard_group', 'target_type', 'environment', 'host_or_endpoint', 'port', 'protocol_type', 'config_payload', 'current_status', 'is_maintenance', 'last_checked_at']);

        // Redis keeps numbers as plain strings and returns them uncast, so the
        // cached figure is normalized back to a number before it reaches the page.
        $cachedSla = Cache::remember('noc:sla:global', 60, fn () => $this->globalSla());

        return Inertia::render('Dashboard/NocOverview', [
            'summary' => $this->buildSummary($targets),
            'targets' => $this->decorateTargets($targets),
            'incidents' => $this->activeIncidents(),
            'globalSla' => $cachedSla === null ? null : (float) $cachedSla,
            'slaWindowDays' => now()->setTimezone('Asia/Jakarta')->daysInMonth,
            'slaTrend' => Cache::remember('noc:sla:trend', 60, fn () => $this->slaTrend()),
            'targetDailySla' => Cache::remember('noc:sla:target-daily', 60, fn () => $this->targetDailySla()),
            'slaDate' => $this->latestLogDate(),
        ]);
    }

    /**
     * Alerts index: filterable incident list (open / resolved / all).
     */
    public function alerts(Request $request): Response
    {
        $filter = $request->query('filter', 'open');
        if (! in_array($filter, ['open', 'resolved', 'all'], true)) {
            $filter = 'open';
        }

        $query = Incident::with('target:id,name')->orderByDesc('started_at')->limit(100);
        if ($filter === 'open') {
            $query->open();
        } elseif ($filter === 'resolved') {
            $query->where('status', 'RESOLVED');
        }

        $incidents = $query->get()->map(fn ($i) => [
            'id' => $i->id,
            'target_id' => $i->target_id,
            'target_name' => $i->target?->name,
            'severity' => $i->severity,
            'status' => $i->status,
            'root_cause_message' => $i->root_cause_message,
            'started_at' => $i->started_at?->toIso8601String(),
            'resolved_at' => $i->resolved_at?->toIso8601String(),
        ])->all();

        return Inertia::render('Dashboard/AlertsIndex', [
            'incidents' => $incidents,
            'filter' => $filter,
        ]);
    }

    /**
     * Dashboard 1: Network devices.
     */
    public function network(): Response
    {
        $targets = MonitoringTarget::where('is_active', true)
            ->forDashboard('NETWORK')
            ->get();

        return Inertia::render('Dashboard/NetworkDashboard', [
            'targets' => $this->decorateTargets($targets, true),
            'internetTargets' => $this->decorateTargets($targets->where('target_type', 'INTERNET')->values(), true),
            'trafficSeries' => $this->trafficSeries($targets),
        ]);
    }

    /**
     * Dashboard 2: Applications, Domains/SSL, Databases.
     */
    public function appsDomainsDb(Request $request): Response
    {
        $environment = $request->query('environment', 'ALL');

        $base = MonitoringTarget::where('is_active', true)->forDashboard('APP_DOMAIN_DB');

        // Tab counts come from the same scope as the list, so a tab never claims a
        // count the list cannot show.
        $counts = [
            'ALL' => (clone $base)->count(),
            'PRODUCTION' => (clone $base)->where('environment', 'PRODUCTION')->count(),
            'STAGING' => (clone $base)->where('environment', 'STAGING')->count(),
            'DEVELOPMENT' => (clone $base)->where('environment', 'DEVELOPMENT')->count(),
        ];

        $query = clone $base;
        if ($environment !== 'ALL') {
            $query->where('environment', $environment);
        }

        $targets = $query->get();
        return Inertia::render('Dashboard/AppDbDomainDashboard', [
            'targets' => $this->decorateTargets($targets, true),
            'environment' => $environment,
            'counts' => $counts,
        ]);
    }

    /**
     * Dashboard 3: Mail, S3, Beszel.
     */
    public function mailS3Beszel(): Response
    {
        $targets = MonitoringTarget::where('is_active', true)
            ->forDashboard('MAIL_S3_BESZEL')
            ->get();

        return Inertia::render('Dashboard/MailS3BeszelDashboard', [
            'targets' => $this->decorateTargets($targets, true),
        ]);
    }

    /**
     * Lightweight polling endpoint: reads Redis only, no DB polling.
     */
    public function states()
    {
        $targets = MonitoringTarget::where('is_active', true)
            ->get(['id', 'current_status', 'last_checked_at']);
        $statuses = $targets->mapWithKeys(fn ($t) => [
            $t->id => [
                'current_status' => $t->current_status,
                'last_checked_at' => $t->last_checked_at?->toIso8601String(),
            ],
        ]);
        return response()->json(['statuses' => $statuses]);
    }

    /**
     * @param  \Illuminate\Support\Collection<int, MonitoringTarget>  $targets
     */
    private function buildSummary($targets): array
    {
        $counts = $targets->groupBy('current_status')->map->count();

        return [
            'total' => $targets->count(),
            'up' => $counts->get('UP', 0),
            'degraded' => $counts->get('DEGRADED', 0),
            'down' => $counts->get('DOWN', 0),
            'maintenance' => $counts->get('MAINTENANCE', 0),
        ];
    }


    /**
     * @param  \Illuminate\Support\Collection<int, MonitoringTarget>  $targets
     */
    private function decorateTargets($targets, bool $withState = false): array
    {
        return $targets->map(function (MonitoringTarget $target) use ($withState) {
            $row = [
                'id' => $target->id,
                'name' => $target->name,
                'dashboard_group' => $target->dashboard_group,
                'target_type' => $target->target_type,
                'environment' => $target->environment,
                'host_or_endpoint' => $target->host_or_endpoint,
                'port' => $target->port,
                'protocol_type' => $target->protocol_type,
                'config_payload' => $target->config_payload,
                'current_status' => $target->current_status,
                'is_maintenance' => $target->is_maintenance,
                'last_checked_at' => $target->last_checked_at?->toIso8601String(),
            ];
            if ($withState) {
                $row['state'] = $this->state->get($target->id);
            }
            return $row;
        })->all();
    }


    private function activeIncidents(): array
    {
        return Incident::with('target:id,name')
            ->open()
            ->orderByDesc('started_at')
            ->limit(20)
            ->get()
            ->map(fn ($i) => [
                'id' => $i->id,
                'target_id' => $i->target_id,
                'target_name' => $i->target?->name,
                'severity' => $i->severity,
                'status' => $i->status,
                'root_cause_message' => $i->root_cause_message,
                'started_at' => $i->started_at?->toIso8601String(),
            ])
            ->all();
    }

    /**
     * Pooled monthly SLA across active targets, or null when no log exists yet.
     * Returning null lets the UI say "no data" instead of showing a fabricated 100%.
     */
    private function globalSla(): ?float
    {
        $now = now()->setTimezone('Asia/Jakarta');
        $start = $now->copy()->startOfMonth();
        $end = $now->copy()->endOfMonth();

        // Maintenance empty for now; manual schedule input fills the map later.
        $result = $this->sla->forWindow($start, $end);

        return $result['sla'] === null ? null : round($result['sla'], 2);
    }

    /**
     * Daily SLA rows for the NOC trend chart, last 30 calendar days oldest first.
     * Each day is one pooled forWindow call, so no-data days carry null uptime
     * instead of a padded 100. Latency and incidents keep their existing queries.
     *
     * @return array<int, array{date: string, uptime: float|null, avg_latency: float|null, incidents: int}>
     */
    private function slaTrend(): array
    {
        $today = now()->setTimezone('Asia/Jakarta')->startOfDay();
        $incidents = DB::table('incidents')
            ->where('started_at', '>=', $today->copy()->subDays(29))
            ->groupBy('day')
            ->selectRaw("to_char(started_at, 'YYYY-MM-DD') AS day, COUNT(*) AS total")
            ->pluck('total', 'day');

        $points = [];
        for ($i = 29; $i >= 0; $i--) {
            $day = $today->copy()->subDays($i);
            $dayStart = $day->copy()->startOfDay();
            $dayEnd = $day->copy()->endOfDay();

            // Maintenance empty for now; manual schedule input fills the map later.
            $agg = $this->sla->forWindow($dayStart, $dayEnd);

            $latency = DB::table('monitoring_logs')
                ->whereBetween('checked_at', [$dayStart, $dayEnd])
                ->avg('response_time_ms');

            $points[] = [
                'date' => $day->toDateString(),
                'uptime' => $agg['sla'] === null ? null : round($agg['sla'], 2),
                'avg_latency' => $latency === null ? null : round((float) $latency, 1),
                'incidents' => (int) ($incidents[$day->toDateString()] ?? 0),
            ];
        }

        return $points;
    }

    /**
     * Latest log date, or null when no log exists yet.
     */
    private function latestLogDate(): ?string
    {
        $max = DB::table('monitoring_logs')->max('checked_at');

        return $max === null ? null : now()->parse($max)->setTimezone('Asia/Jakarta')->toDateString();
    }

    /**
     * Per-target SLA rows for the latest day with log data, worst uptime first.
     *
     * @return array<int, array{target_id: string, name: string, current_status: string, uptime: float|null, total_checks: int, failed_checks: int}>
     */
    private function targetDailySla(): array
    {
        $latest = $this->latestLogDate();

        if ($latest === null) {
            return [];
        }

        $day = now()->setTimezone('Asia/Jakarta')->parse($latest);
        $dayStart = $day->copy()->startOfDay();
        $dayEnd = $day->copy()->endOfDay();

        // Maintenance empty for now; manual schedule input fills the map later.
        $result = $this->sla->forWindow($dayStart, $dayEnd);
        $byTarget = collect($result['targets'])->keyBy('target_id');

        return MonitoringTarget::where('is_active', true)
            ->orderBy('name')
            ->get(['id', 'name', 'current_status'])
            ->map(function (MonitoringTarget $target) use ($byTarget, $dayStart, $dayEnd) {
                $row = $byTarget->get($target->id);
                $counts = DB::table('monitoring_logs')
                    ->where('target_id', $target->id)
                    ->whereBetween('checked_at', [$dayStart, $dayEnd])
                    ->selectRaw("COUNT(*) AS total, COUNT(*) FILTER (WHERE status = 'DOWN') AS down_checks")
                    ->first();

                return [
                    'target_id' => $target->id,
                    'name' => $target->name,
                    'current_status' => $target->current_status,
                    'uptime' => $row === null || $row['sla'] === null ? null : round($row['sla'], 2),
                    'total_checks' => (int) ($counts->total ?? 0),
                    'failed_checks' => (int) ($counts->down_checks ?? 0),
                ];
            })
            ->filter(fn ($row) => $row['uptime'] !== null)
            ->sortBy(fn ($row) => $row['uptime'])
            ->values()
            ->all();
    }
    /**
     * SNMP traffic per device over the last 24 hours, bucketed to five minutes.
     * Numbers come from monitoring_logs, so a bucket the probe never filled stays
     * null and the chart shows a gap instead of a line drawn through it.
     *
     * @return array{times: array<int, string>, devices: array<int, array<string, mixed>>, total: int, truncated: bool}
     */
    private function trafficSeries($targets): array
    {
        $bucketMinutes = self::TRAFFIC_BUCKET_MINUTES;
        $now = now()->setTimezone(config('app.timezone'));

        // A rolling 24 h window on a five-minute grid, so every label lands on a
        // boundary and the newest point is the bucket still in progress.
        $windowEnd = $now->copy()->startOfMinute();
        $windowEnd->setTime(
            (int) $windowEnd->format('G'),
            (int) $windowEnd->format('i') - ((int) $windowEnd->format('i') % $bucketMinutes),
        );
        $windowStart = $windowEnd->copy()->subDay();

        $buckets = [];
        $labels = [];

        for ($cursor = $windowStart->copy(); $cursor->lessThanOrEqualTo($windowEnd); $cursor->addMinutes($bucketMinutes)) {
            $buckets[] = $cursor->format('Y-m-d\TH:i');
            $labels[] = $cursor->format('H:i');
        }

        $targetIds = $targets->pluck('id')->all();

        if ($targetIds === []) {
            return ['times' => $labels, 'devices' => [], 'total' => 0, 'truncated' => false];
        }

        // Buckets are cut in the app timezone and rendered as a sortable key, so a
        // session or server timezone setting cannot shift a point to another hour.
        $bucketExpression = "to_char(date_bin(INTERVAL '{$bucketMinutes} minutes', checked_at AT TIME ZONE 'Asia/Jakarta', TIMESTAMP '2000-01-01'), 'YYYY-MM-DD\"T\"HH24:MI')";

        $rows = DB::table('monitoring_logs')
            ->selectRaw("target_id, {$bucketExpression} AS bucket, avg((metrics_payload->>'rx_mbps')::numeric) AS rx, avg((metrics_payload->>'tx_mbps')::numeric) AS tx")
            ->whereIn('target_id', $targetIds)
            // An ISO string keeps the bound absolute; a formatted datetime would be
            // read in the connection's timezone and move the window by its offset.
            ->where('checked_at', '>=', $windowStart->toIso8601String())
            ->whereRaw("(metrics_payload->>'rx_mbps') IS NOT NULL OR (metrics_payload->>'tx_mbps') IS NOT NULL")
            ->groupByRaw("target_id, {$bucketExpression}")
            ->get();

        $bucketIndex = array_flip($buckets);
        $byTarget = [];

        foreach ($rows as $row) {
            $index = $bucketIndex[$row->bucket] ?? null;

            if ($index === null) {
                continue;
            }

            $byTarget[(string) $row->target_id][$index] = [
                'rx' => $row->rx === null ? null : round((float) $row->rx, 3),
                'tx' => $row->tx === null ? null : round((float) $row->tx, 3),
            ];
        }

        $devices = [];

        foreach ($targets as $target) {
            $readings = $byTarget[(string) $target->id] ?? null;

            if ($readings === null) {
                continue;
            }

            $rx = array_fill(0, count($buckets), null);
            $tx = array_fill(0, count($buckets), null);
            $peak = 0.0;

            foreach ($readings as $index => $values) {
                $rx[$index] = $values['rx'];
                $tx[$index] = $values['tx'];
                $peak = max($peak, (float) $values['rx'], (float) $values['tx']);
            }

            $devices[] = ['name' => $target->name, 'rx' => $rx, 'tx' => $tx, 'peak_mbps' => $peak];
        }

        // Busiest first, so a shortened list keeps the devices that answer the
        // question the chart asks.
        usort($devices, fn (array $a, array $b) => [$b['peak_mbps'], $a['name']] <=> [$a['peak_mbps'], $b['name']]);

        $total = count($devices);
        $devices = array_slice($devices, 0, self::TRAFFIC_DEVICE_LIMIT);

        return [
            'times' => $labels,
            'devices' => array_map(
                fn (array $device) => ['name' => $device['name'], 'rx' => $device['rx'], 'tx' => $device['tx']],
                $devices,
            ),
            'total' => $total,
            'truncated' => $total > self::TRAFFIC_DEVICE_LIMIT,
        ];
    }
}
