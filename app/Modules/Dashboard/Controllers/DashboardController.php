<?php

namespace App\Modules\Dashboard\Controllers;

use App\Modules\Incidents\Models\Incident;
use App\Modules\Probes\Services\TargetStateService;
use App\Modules\Targets\Models\MonitoringTarget;
use Illuminate\Http\Request;
use Illuminate\Support\Facades\DB;
use Inertia\Inertia;
use Inertia\Response;

class DashboardController
{
    public function __construct(private readonly TargetStateService $state)
    {
    }

    /**
     * Executive NOC Overview: all targets grouped, global SLA.
     */
    public function noc(): Response
    {
        $targets = MonitoringTarget::where('is_active', true)->get();

        return Inertia::render('Dashboard/NocOverview', [
            'summary' => $this->buildSummary($targets),
            'targets' => $this->decorateTargets($targets),
            'incidents' => $this->activeIncidents(),
            'globalSla' => $this->globalSla(),
            'slaWindowDays' => 30,
            'slaTrend' => $this->slaTrend(),
            'targetDailySla' => $this->targetDailySla(),
            'slaDate' => $this->latestSummaryDate(),
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
            'targets' => $this->decorateTargets($targets),
            'internetTargets' => $this->decorateTargets($targets->where('target_type', 'INTERNET')->values()),
            'trafficSummary' => $this->trafficSummary($targets),
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
            'targets' => $this->decorateTargets($targets),
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
            'targets' => $this->decorateTargets($targets),
        ]);
    }

    /**
     * Lightweight polling endpoint: reads Redis only, no DB polling.
     */
    public function states()
    {
        $targets = MonitoringTarget::where('is_active', true)
            ->get(['id', 'name', 'current_status']);

        $states = $targets->mapWithKeys(fn ($t) => [
            $t->id => $this->state->get($t->id),
        ]);

        return response()->json(['states' => $states]);
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
    private function decorateTargets($targets): array
    {
        return $targets->map(function (MonitoringTarget $target) {
            $state = $this->state->get($target->id);

            return [
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
                'state' => $state,
                'heartbeat' => $this->heartbeat($target->id),
            ];
        })->all();
    }

    /**
     * Last 60 check outcomes, oldest first.
     *
     * A slot with no check is returned as null so the UI can render an empty slot.
     * Padding with a fabricated 'UP' would invent 60 healthy checks that never
     * happened (R-17, R-38).
     *
     * @return array<int, string|null>
     */
    private function heartbeat(string $targetId): array
    {
        $logs = DB::table('monitoring_logs')
            ->where('target_id', $targetId)
            ->orderByDesc('checked_at')
            ->limit(60)
            ->pluck('status')
            ->all();

        // Oldest -> newest, pad the front to 60 with nulls
        $logs = array_reverse($logs);
        $pad = array_fill(0, max(0, 60 - count($logs)), null);

        return array_slice([...$pad, ...$logs], -60);
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
     * Average uptime over the window, or null when no daily rollup exists yet.
     * Returning null lets the UI say "no data" instead of showing a fabricated 100%
     * (R-17, R-38).
     */
    private function globalSla(): ?float
    {
        $avg = DB::table('monitoring_daily_summaries')
            ->where('summary_date', '>=', now()->subDays(30))
            ->avg('uptime_percentage');

        return $avg === null ? null : round((float) $avg, 2);
    }

    /**
     * Daily SLA rows for the NOC trend chart, one entry per rollup day for the
     * last 30 days, oldest first. The summary table holds one row per target per
     * day, so uptime and latency are averaged across the targets that reported
     * that day. A day with no rollup row is absent instead of padded, and a day
     * without an incident gets a real 0 (R-17, R-38).
     *
     * @return array<int, array{date: string, uptime: float|null, avg_latency: float|null, incidents: int}>
     */
    private function slaTrend(): array
    {
        $rows = DB::table('monitoring_daily_summaries')
            ->where('summary_date', '>=', now()->subDays(30))
            ->groupBy('summary_date')
            ->orderBy('summary_date')
            ->selectRaw('summary_date, AVG(uptime_percentage) AS uptime, AVG(avg_response_time_ms) AS avg_latency')
            ->get();

        if ($rows->isEmpty()) {
            return [];
        }

        $incidents = DB::table('incidents')
            ->where('started_at', '>=', now()->subDays(30))
            ->groupBy('day')
            ->selectRaw("to_char(started_at, 'YYYY-MM-DD') AS day, COUNT(*) AS total")
            ->pluck('total', 'day');

        return $rows->map(fn ($row) => [
            'date' => (string) $row->summary_date,
            'uptime' => $row->uptime === null ? null : round((float) $row->uptime, 2),
            'avg_latency' => $row->avg_latency === null ? null : round((float) $row->avg_latency, 1),
            'incidents' => (int) ($incidents[(string) $row->summary_date] ?? 0),
        ])->all();
    }

    /**
     * Latest rollup date, or null when no daily rollup exists yet.
     */
    private function latestSummaryDate(): ?string
    {
        $max = DB::table('monitoring_daily_summaries')->max('summary_date');

        return $max === null ? null : (string) $max;
    }

    /**
     * Per-target SLA rows for the latest rollup day, worst uptime first.
     *
     * @return array<int, array{target_id: string, name: string, current_status: string, uptime: float, total_checks: int, failed_checks: int}>
     */
    private function targetDailySla(): array
    {
        $latest = $this->latestSummaryDate();

        if ($latest === null) {
            return [];
        }

        return DB::table('monitoring_daily_summaries as s')
            ->join('monitoring_targets as t', 't.id', '=', 's.target_id')
            ->where('s.summary_date', $latest)
            ->where('t.is_active', true)
            ->orderBy('s.uptime_percentage')
            ->select('s.target_id', 't.name', 't.current_status', 's.uptime_percentage as uptime', 's.total_checks', 's.failed_checks')
            ->get()
            ->map(fn ($row) => [
                'target_id' => $row->target_id,
                'name' => $row->name,
                'current_status' => $row->current_status,
                'uptime' => round((float) $row->uptime, 2),
                'total_checks' => (int) $row->total_checks,
                'failed_checks' => (int) $row->failed_checks,
            ])
            ->all();
    }

    /**
     * Aggregate SNMP traffic (rx/tx) for the area chart.
     */
    private function trafficSummary($targets): array
    {
        return $targets
            ->mapWithKeys(function (MonitoringTarget $target) {
                $metrics = $this->state->get($target->id)['summary_metrics'] ?? [];

                return [
                    $target->name => [
                        'rx_mbps' => $metrics['rx_mbps'] ?? 0,
                        'tx_mbps' => $metrics['tx_mbps'] ?? 0,
                    ],
                ];
            })
            ->all();
    }
}
