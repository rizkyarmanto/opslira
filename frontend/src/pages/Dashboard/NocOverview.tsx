import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import ReactECharts from 'echarts-for-react';
import { AlertTriangle, ArrowRight, CheckCircle2, Inbox, XOctagon } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Button from '@/components/shared/Button';
import Card from '@/components/shared/Card';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import StatusBadge, { StatusRail } from '@/components/shared/StatusBadge';
import SlidingNumber from '@/components/motion-primitives/SlidingNumber';
import { Head } from '@/lib/head';
import { relativeTime } from '@/lib/time';
import type {
    Incident,
    MonitoringTarget,
    NocSummary,
    TargetStatus,
} from '@/types/monitoring';

interface SlaTrendPoint {
    date: string;
    uptime: number | null;
    avg_latency: number | null;
    incidents: number;
}

interface TargetDailySla {
    target_id: string;
    name: string;
    current_status: TargetStatus;
    uptime: number;
    total_checks: number;
    failed_checks: number;
}

type ChartTab = 'uptime' | 'latency';

/** Bar fill for a daily SLA row, keyed by the target's own current_status.
 *  Status is semantic data (DESIGN.md); the percentage never picks the color. */
const slaBarFill: Record<TargetStatus, string> = {
    UP: '#34d399',
    DEGRADED: '#fbbf24',
    DOWN: '#fb7185',
    MAINTENANCE: '#38bdf8',
    PENDING: 'transparent',
};

interface Props {
    summary: NocSummary & { maintenance: number };
    targets: MonitoringTarget[];
    incidents: Incident[];
    globalSla: number | null;
    slaWindowDays: number | null;
    slaTrend: SlaTrendPoint[] | null;
    targetDailySla: TargetDailySla[] | null;
    slaDate: string | null;
    /** Live status overrides from GET /dashboard/states, keyed by target id. */
    statuses: Record<string, { current_status: TargetStatus; last_checked_at: string | null }>;
    /** The full NOC payload is being refetched on its 60 second interval. */
    refreshing: boolean;
    /** Time the status query last succeeded, for the "Updated" label. */
    lastUpdated: Date | null;
}

/**
 * NOC overview. The screen answers one question: what needs attention right now?
 * So the hierarchy leads with the targets that are not healthy and the open
 * incidents, and the totals sit underneath as context (C-3, R-20).
 */
export default function NocOverview({
    summary,
    targets,
    incidents,
    globalSla,
    slaWindowDays,
    slaTrend,
    targetDailySla,
    slaDate,
    statuses,
    refreshing,
    lastUpdated,
}: Props) {
    // Redis-backed statuses arrive from their own 15 second query; the NOC payload
    // behind them refreshes every 60 seconds, so the freshest value wins per id.
    const liveTargets = useMemo(
        () =>
            targets.map((target) =>
                statuses[target.id]
                    ? {
                          ...target,
                          current_status: statuses[target.id].current_status,
                          last_checked_at: statuses[target.id].last_checked_at,
                      }
                    : target,
            ),
        [targets, statuses],
    );
    const [showAllTargets, setShowAllTargets] = useState(false);
    const [chartTab, setChartTab] = useState<ChartTab>('uptime');
    const [selectedDate, setSelectedDate] = useState<string | null>(null);
    const liveStatusById = new Map(liveTargets.map((t) => [t.id, t]));
    const orderedIncidents = [...incidents].sort((a, b) => {
        const bySeverity = (a.severity === 'CRITICAL' ? 0 : 1) - (b.severity === 'CRITICAL' ? 0 : 1);
        if (bySeverity !== 0) return bySeverity;
        return b.started_at.localeCompare(a.started_at);
    });
    const incidentTotal = orderedIncidents.length;
    const slaPoints = slaTrend ?? [];
    const dailySla = targetDailySla ?? [];
    const hasSlaTrend = slaPoints.length > 0;
    const chartTabs = [
        { key: 'uptime' as ChartTab, label: 'Uptime %', available: hasSlaTrend },
        { key: 'latency' as ChartTab, label: 'Latency ms', available: hasSlaTrend },
    ].filter((tab) => tab.available);
    const activeTab: ChartTab = chartTabs.some((tab) => tab.key === chartTab)
        ? chartTab
        : (chartTabs[0]?.key ?? 'uptime');
    const visibleIncidents = selectedDate
        ? orderedIncidents.filter((i) => i.started_at.slice(0, 10) === selectedDate)
        : orderedIncidents.slice(0, 5);
    return (
        <AppLayout title="NOC Overview">
            <Head title="NOC Overview" />

            <div className="mb-5 flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <p className="text-xs text-noc-muted">
                    {refreshing ? (
                        <ReloadIndicator label="Refreshing status" />
                    ) : lastUpdated ? (
                        `Updated ${lastUpdated.toLocaleTimeString('en-US')}`
                    ) : (
                        'Loading latest status...'
                    )}
                </p>
                <p className="text-xs text-noc-muted">Status badges refresh every 15 seconds. Full data refreshes every 60 seconds.</p>
            </div>

            {/* Executive strip: the four verdict numbers in one row. Needs attention
                leads and carries the largest figure, so the screen's one focal point
                reads first (C-3, ENERGY 2). Three cells are real links to the list
                that answers them; the SLA cell is display-only because no filtered
                view exists for it (R-24, R-26). Icons plus labels pair every value,
                so color is never the only carrier. */}
            <section aria-label="Status summary" className="mb-6">
                <Card className="grid grid-cols-2 gap-px overflow-hidden bg-noc-border md:grid-cols-4">
                    <Link
                        to="/targets?status=DOWN"
                        aria-label="View DOWN targets"
                        className="bg-noc-surface px-4 py-3 transition-colors hover:bg-white/[0.03]"
                    >
                        <div className="flex items-center gap-1.5">
                            <AlertTriangle size={13} className="text-status-degraded" aria-hidden />
                            <span className="field-label">Needs attention</span>
                        </div>
                        <p className="tabular mt-1 text-3xl font-semibold text-status-down">
                            {summary.down + summary.degraded}
                        </p>
                    </Link>
                    <Link
                        to="/targets?status=UP"
                        aria-label="View UP targets"
                        className="bg-noc-surface px-4 py-3 transition-colors hover:bg-white/[0.03]"
                    >
                        <div className="flex items-center gap-1.5">
                            <CheckCircle2 size={13} className="text-status-up" aria-hidden />
                            <span className="field-label">UP</span>
                        </div>
                        <p className="tabular mt-1 text-2xl font-semibold text-status-up">
                            <SlidingNumber value={summary.up} />
                        </p>
                    </Link>
                    <Link
                        to="/dashboard/alerts"
                        aria-label="View open incidents"
                        className="bg-noc-surface px-4 py-3 transition-colors hover:bg-white/[0.03]"
                    >
                        <div className="flex items-center gap-1.5">
                            <XOctagon size={13} className="text-status-down" aria-hidden />
                            <span className="field-label">Open incidents</span>
                        </div>
                        <p className="tabular mt-1 text-2xl font-semibold text-status-down">
                            <SlidingNumber value={incidents.length} />
                        </p>
                    </Link>
                    <div className="bg-noc-surface px-4 py-3">
                        <div className="flex items-center gap-1.5">
                            <CheckCircle2 size={13} className="text-status-up" aria-hidden />
                            <span className="field-label">
                                SLA {slaWindowDays ? `last ${slaWindowDays} days` : ''} (incl. today)
                            </span>
                        </div>
                        {globalSla === null ? (
                            <p className="mt-1 text-sm text-noc-muted">
                                No daily data yet. SLA appears after the first rollup runs.
                            </p>
                        ) : (
                            <p className="tabular mt-1 text-2xl font-semibold text-status-up">
                                <SlidingNumber value={globalSla} decimals={2} suffix="%" />
                            </p>
                        )}
                    </div>
                </Card>
            </section>

            {/* Daily trend: the rollup's answer to how the fleet has been doing */}
            <section className="mb-8" aria-label="Daily trend">
                <Card className="px-4 py-4">
                    {!hasSlaTrend ? (
                        <p className="py-6 text-center text-sm text-noc-muted">
                            No daily data yet. Trends appear after the rollup runs.
                        </p>
                    ) : (
                        <>
                            <div className="mb-2 flex flex-wrap items-center justify-between gap-3">
                                <h2 className="text-sm font-semibold text-noc-text">
                                    {activeTab === 'uptime' && 'Uptime trend, last 30 days (today live)'}
                                    {activeTab === 'latency' && 'Latency trend, last 30 days (today live)'}
                                </h2>
                                <div className="flex items-center gap-1">
                                    {chartTabs.map((tab) => (
                                        <button
                                            key={tab.key}
                                            type="button"
                                            aria-pressed={activeTab === tab.key}
                                            onClick={() => setChartTab(tab.key)}
                                            className={`rounded-xs px-2.5 py-1 text-xs transition-colors ${
                                                activeTab === tab.key
                                                    ? 'bg-white/5 font-medium text-noc-text'
                                                    : 'text-noc-muted hover:text-noc-text'
                                            }`}
                                        >
                                            {tab.label}
                                        </button>
                                    ))}
                                </div>
                            </div>
                            <ReactECharts
                                key={activeTab}
                                option={buildTrendOption(activeTab, slaPoints)}
                                style={{ height: 260 }}
                                onEvents={{
                                    click: (params: { name?: unknown }) => {
                                        const day =
                                            typeof params?.name === 'string' ? params.name : '';
                                        if (/^\d{4}-\d{2}-\d{2}$/.test(day)) {
                                            setSelectedDate(day);
                                        }
                                    },
                                }}
                            />
                        </>
                    )}
                </Card>
            </section>

            {/* System SLA: the rollup's per-target verdict for one calendar day.
                Rows come sorted worst-first from the backend, so the target most
                likely to need attention reads first (C-3). The bar fill names the
                target's real status, never a threshold invented here. */}
            <section className="mb-8" aria-labelledby="system-sla-heading">
                <div className="mb-3 flex flex-wrap items-baseline gap-2">
                    <h2 id="system-sla-heading" className="text-sm font-semibold text-noc-text">
                        {slaDate ? `System SLA by target (daily rollup), ${slaDate}` : 'System SLA by target (daily rollup)'}
                    </h2>
                    <span className="text-xs text-noc-muted">Status badges update every 15 seconds. Percentages update daily.</span>
                </div>
                {dailySla.length === 0 ? (
                    <Card className="px-4 py-6">
                        <p className="text-sm text-noc-muted">
                            No daily data yet. Per-target SLA appears after the rollup runs.
                        </p>
                    </Card>
                ) : (
                    <div className="space-y-2">
                        {dailySla.map((row) => (
                            <Link
                                key={row.target_id}
                                to={`/targets/${row.target_id}`}
                                className="flex items-center gap-4 rounded-md border border-noc-border bg-noc-surface px-4 py-3 transition-colors hover:bg-white/[0.03]"
                            >
                                <div className="flex min-w-0 items-center gap-3 sm:w-72">
                                    <p className="truncate text-sm font-medium text-noc-text">{row.name}</p>
                                    <StatusBadge status={row.current_status} />
                                </div>
                                <div className="flex h-6 flex-1 items-center">
                                    <div className="h-2 w-full overflow-hidden rounded-xs bg-white/5">
                                        <div
                                            className="h-full rounded-xs"
                                            style={{
                                                width: `${row.uptime}%`,
                                                backgroundColor: slaBarFill[row.current_status],
                                            }}
                                        />
                                    </div>
                                </div>
                                <span className="tabular w-24 shrink-0 text-right text-sm text-noc-text">
                                    {row.uptime.toFixed(2)}%
                                </span>
                                <span className="tabular hidden w-28 shrink-0 text-right text-xs text-noc-muted sm:block">
                                    {row.total_checks} checks
                                </span>
                            </Link>
                        ))}
                    </div>
                )}
            </section>

            {/* Focal point: what is not healthy */}
            <section className="mb-8" aria-labelledby="attention-heading">
                <div className="mb-3 flex items-center gap-2">
                    <AlertTriangle
                        size={16}
                        className={visibleIncidents.length > 0 ? 'text-status-down' : 'text-noc-muted'}
                        aria-hidden
                    />
                    <h2 id="attention-heading" className="text-sm font-semibold text-noc-text">
                        Needs attention
                    </h2>
                    <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                        {visibleIncidents.length}
                    </span>
                    {!selectedDate && incidentTotal > 5 && (
                        <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                            Showing 5 of {incidentTotal}
                        </span>
                    )}
                    {selectedDate && (
                        <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                            Showing {selectedDate}
                        </span>
                    )}
                    {selectedDate && (
                        <Button
                            variant="ghost"
                            className="ml-auto px-2.5 py-1 text-xs"
                            onClick={() => setSelectedDate(null)}
                        >
                            Clear
                        </Button>
                    )}
                </div>

                {visibleIncidents.length === 0 ? (
                    <Card className="flex items-center gap-3 px-4 py-4">
                        <Inbox size={16} className="text-noc-muted" aria-hidden />
                        <p className="text-sm text-noc-muted">No open incidents.</p>
                    </Card>
                ) : (
                    <div className="space-y-2">
                        {visibleIncidents.map((incident) => {
                            const liveTarget = liveStatusById.get(incident.target_id);
                            const cardStatus =
                                liveTarget?.current_status ??
                                (incident.severity === 'CRITICAL' ? 'DOWN' : 'DEGRADED');
                            return (
                                <Link
                                    key={incident.id}
                                    to={`/targets/${incident.target_id}`}
                                    className="group relative flex items-center gap-4 overflow-hidden rounded-md border border-noc-border bg-noc-surface px-4 py-3 pl-5 hover:border-zinc-600"
                                >
                                    <StatusRail status={cardStatus} />
                                    <span
                                        className={`mt-0.5 shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold ${
                                            incident.severity === 'CRITICAL'
                                                ? 'status-down'
                                                : 'status-degraded'
                                        }`}
                                    >
                                        {incident.severity === 'CRITICAL' ? 'CRITICAL' : 'WARNING'}
                                    </span>
                                    <StatusBadge status={cardStatus} />
                                    <div className="min-w-0 flex-1">
                                        <p className="truncate text-sm font-medium text-noc-text">
                                            {incident.target_name ?? liveTarget?.name ?? 'Unknown target'}
                                        </p>
                                        <p className="text-xs text-noc-muted">{incident.root_cause_message}</p>
                                    </div>
                                    <span className="tabular hidden shrink-0 text-xs text-noc-muted sm:block">
                                        {relativeTime(incident.started_at) ?? ''}
                                    </span>
                                    <ArrowRight
                                        size={15}
                                        className="shrink-0 text-noc-muted group-hover:text-noc-text"
                                        aria-hidden
                                    />
                                </Link>
                            );
                        })}
                    </div>
                )}
                <div className="mt-3">
                    <Link to="/dashboard/alerts" className="text-xs text-noc-accent hover:underline">
                        View all alerts
                    </Link>
                </div>
            </section>
            {/* Full inventory */}
            <section aria-labelledby="all-targets-heading">
                <div className="mb-3 flex items-center gap-2">
                    <h2 id="all-targets-heading" className="text-sm font-semibold text-noc-text">
                        All targets
                    </h2>
                    <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                        {targets.length}
                    </span>
                </div>

                {targets.length === 0 ? (
                    <Card className="px-4 py-8 text-center">
                        <p className="text-sm text-noc-muted">
                            No targets registered yet. Add your first target on the Targets page.
                        </p>
                        <Link to="/targets" className="btn-primary mt-3">
                            Open the Targets page
                        </Link>
                    </Card>
                ) : (
                    <>
                        <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                            {(showAllTargets ? targets : targets.slice(0, 12)).map((target) => (
                                <Link
                                    key={target.id}
                                    to={`/targets/${target.id}`}
                                    className="relative flex items-center gap-3 overflow-hidden rounded-md border border-noc-border bg-noc-surface px-4 py-3 pl-5 hover:border-zinc-600"
                                >
                                    <StatusRail status={target.current_status} />
                                    <div className="min-w-0 flex-1">
                                        <p className="truncate text-sm font-medium text-noc-text">{target.name}</p>
                                        <p className="tabular truncate text-xs text-noc-muted">
                                            {target.host_or_endpoint}
                                            {target.port ? `:${target.port}` : ''}
                                        </p>
                                    </div>
                                    <StatusBadge status={target.current_status} />
                                </Link>
                            ))}
                        </div>
                        {!showAllTargets && targets.length > 12 && (
                            <Button
                                variant="ghost"
                                className="mt-3"
                                onClick={() => setShowAllTargets(true)}
                            >
                                Show all {targets.length} targets
                            </Button>
                        )}
                    </>
                )}
            </section>
        </AppLayout>
    );
}

interface TrendTooltipParam {
    seriesName: string;
    value: number | null;
    axisValueLabel: string;
}

/** Tooltip body shared by every tab: the day, then each series' exact reading. */
function trendTooltip(params: TrendTooltipParam[] | TrendTooltipParam, format: (v: number) => string) {
    const list = Array.isArray(params) ? params : [params];
    const day = list[0]?.axisValueLabel ?? '';
    const lines = list.map((p) => `${p.seriesName}: ${typeof p.value === 'number' ? format(p.value) : 'n/a'}`);
    return [day, ...lines].join('<br/>');
}

/**
 * Option for the daily trend chart. One question per tab, only series the
 * backend returned, and the line axes start at the data minimum so a 99% to
 * 100% range reads as a trend instead of a flat line pinned to zero (C-3,
 * R-17). Animation plays once when a tab mounts the chart and never on the
 * 15-second polling update, so motion marks a state change and stops
 * (DESIGN.md, R-19, MOTION 1).
 */
function buildTrendOption(tab: ChartTab, slaPoints: SlaTrendPoint[]) {
    const shared = {
        backgroundColor: 'transparent',
        animation: true,
        animationDuration: 350,
        animationDurationUpdate: 0,
        grid: { left: 48, right: 20, top: 36, bottom: 36 },
        tooltip: {
            trigger: 'axis',
            backgroundColor: '#131316',
            borderColor: '#27272a',
            textStyle: { color: '#f4f4f5', fontSize: 12 },
        },
        legend: {
            textStyle: { color: '#a1a1aa', fontSize: 11 },
            top: 0,
            icon: 'roundRect',
        },
        xAxis: {
            type: 'category' as const,
            data: [] as string[],
            axisLine: { lineStyle: { color: '#3f3f46' } },
            axisLabel: { color: '#a1a1aa', fontSize: 10, hideOverlap: true },
        },
        yAxis: {
            type: 'value' as const,
            nameTextStyle: { color: '#a1a1aa', fontSize: 10 },
            axisLabel: { color: '#a1a1aa', fontSize: 10 },
            splitLine: { lineStyle: { color: '#27272a' } },
        },
    };

    const uptime = tab === 'uptime';
    const seriesName = uptime ? 'Uptime %' : 'Latency ms';
    const color = uptime ? '#34d399' : '#38bdf8';

    return {
        ...shared,
        tooltip: {
            ...shared.tooltip,
            formatter: (params: TrendTooltipParam[] | TrendTooltipParam) =>
                trendTooltip(params, (v) => (uptime ? `${v.toFixed(2)}%` : `${v.toFixed(1)} ms`)),
        },
        legend: { ...shared.legend, data: [seriesName] },
        xAxis: { ...shared.xAxis, data: slaPoints.map((p) => p.date) },
        yAxis: { ...shared.yAxis, name: uptime ? '%' : 'ms', min: 'dataMin' },
        series: [
            {
                name: seriesName,
                type: 'line',
                symbolSize: 5,
                connectNulls: false,
                data: slaPoints.map((p) => (uptime ? p.uptime : p.avg_latency)),
                itemStyle: { color },
                lineStyle: { color },
            },
        ],
    };
}
