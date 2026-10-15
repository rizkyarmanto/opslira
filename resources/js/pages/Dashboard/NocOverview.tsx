import { Head, Link } from '@inertiajs/react';
import { AlertTriangle, ArrowRight, Inbox } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import StatusBadge, { StatusRail } from '@/components/shared/StatusBadge';
import SlidingNumber from '@/components/motion-primitives/SlidingNumber';
import { relativeTime } from '@/lib/time';
import { usePolling } from '@/lib/usePolling';
import type { Incident, MonitoringTarget, NocSummary } from '@/types/monitoring';

interface Props {
    summary: NocSummary & { maintenance: number };
    targets: MonitoringTarget[];
    incidents: Incident[];
    globalSla: number | null;
    slaWindowDays: number | null;
}

/**
 * NOC overview. The screen answers one question: what needs attention right now?
 * So the hierarchy leads with the targets that are not healthy and the open
 * incidents, and the totals sit underneath as context (C-3, R-20).
 */
export default function NocOverview({ summary, targets, incidents, globalSla, slaWindowDays }: Props) {
    const { lastUpdated } = usePolling(['summary', 'targets', 'incidents', 'globalSla']);

    const needsAttention = targets.filter(
        (t) => t.current_status === 'DOWN' || t.current_status === 'DEGRADED',
    );
    const healthy = targets.filter((t) => t.current_status === 'UP');
    const unprobed = targets.filter((t) => t.current_status === 'PENDING');

    return (
        <AppLayout title="NOC Overview">
            <Head title="NOC Overview" />

            <div className="mb-5 flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <p className="text-xs text-noc-muted">
                    {lastUpdated
                        ? `Updated ${lastUpdated.toLocaleTimeString('en-US')}`
                        : 'Loading latest status...'}
                </p>
                <p className="text-xs text-noc-muted">Refreshes every 15 seconds while the tab is active.</p>
            </div>

            {/* Focal point: what is not healthy */}
            <section className="mb-8" aria-labelledby="attention-heading">
                <div className="mb-3 flex items-center gap-2">
                    <AlertTriangle
                        size={16}
                        className={needsAttention.length > 0 ? 'text-status-down' : 'text-noc-muted'}
                        aria-hidden
                    />
                    <h2 id="attention-heading" className="text-sm font-semibold text-noc-text">
                        Needs attention
                    </h2>
                    <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                        {needsAttention.length}
                    </span>
                </div>

                {needsAttention.length === 0 && incidents.length === 0 ? (
                    <Card className="flex items-center gap-3 px-4 py-4">
                        <Inbox size={16} className="text-noc-muted" aria-hidden />
                        <p className="text-sm text-noc-muted">
                            No DOWN or DEGRADED targets, and no open incidents.
                        </p>
                    </Card>
                ) : (
                    <div className="space-y-2">
                        {needsAttention.map((target) => (
                            <Link
                                key={target.id}
                                href={`/targets/${target.id}`}
                                className="group relative flex items-center gap-4 overflow-hidden rounded-md border border-noc-border bg-noc-surface px-4 py-3 pl-5 hover:border-zinc-600"
                            >
                                <StatusRail status={target.current_status} />
                                <StatusBadge status={target.current_status} />
                                <div className="min-w-0 flex-1">
                                    <p className="truncate text-sm font-medium text-noc-text">{target.name}</p>
                                    <p className="tabular truncate text-xs text-noc-muted">
                                        {target.host_or_endpoint}
                                        {target.port ? `:${target.port}` : ''}
                                    </p>
                                </div>
                                <span className="tabular hidden text-xs text-noc-muted sm:block">
                                    {relativeTime(target.last_checked_at) ?? 'Not yet checked'}
                                </span>
                                <ArrowRight
                                    size={15}
                                    className="shrink-0 text-noc-muted group-hover:text-noc-text"
                                    aria-hidden
                                />
                            </Link>
                        ))}

                        {incidents.map((incident) => (
                            <div
                                key={incident.id}
                                className="relative flex items-start gap-4 overflow-hidden rounded-md border border-noc-border bg-noc-surface px-4 py-3 pl-5"
                            >
                                <span
                                    aria-hidden
                                    className={`absolute inset-y-0 left-0 w-[3px] ${
                                        incident.severity === 'CRITICAL' ? 'bg-status-down' : 'bg-status-degraded'
                                    }`}
                                />
                                <span
                                    className={`mt-0.5 shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold ${
                                        incident.severity === 'CRITICAL'
                                            ? 'status-down'
                                            : 'status-degraded'
                                    }`}
                                >
                                    {incident.severity === 'CRITICAL' ? 'CRITICAL' : 'WARNING'}
                                </span>
                                <div className="min-w-0 flex-1">
                                    <p className="truncate text-sm font-medium text-noc-text">
                                        {incident.target_name ?? 'Unknown target'}
                                    </p>
                                    <p className="text-xs text-noc-muted">{incident.root_cause_message}</p>
                                </div>
                                <span className="tabular hidden shrink-0 text-xs text-noc-muted sm:block">
                                    {relativeTime(incident.started_at) ?? ''}
                                </span>
                            </div>
                        ))}
                    </div>
                )}
            </section>

            {/* Context: the totals */}
            <section className="mb-8" aria-labelledby="summary-heading">
                <h2 id="summary-heading" className="mb-3 text-sm font-semibold text-noc-text">
                    Summary
                </h2>
                <Card className="grid grid-cols-2 gap-px overflow-hidden bg-noc-border md:grid-cols-6">
                    <SummaryCell label="Total targets" value={summary.total} />
                    <SummaryCell label="UP" value={summary.up} tone="up" />
                    <SummaryCell label="Degraded" value={summary.degraded} tone="degraded" />
                    <SummaryCell label="Down" value={summary.down} tone="down" />
                    <SummaryCell label="Maintenance" value={summary.maintenance} tone="maintenance" />
                    <SummaryCell label="Not yet checked" value={unprobed.length} tone="pending" />
                </Card>

                <Card className="mt-4 flex flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
                    <div>
                        <p className="field-label">SLA {slaWindowDays ? `last ${slaWindowDays} days` : ''}</p>
                        {globalSla === null ? (
                            <p className="mt-0.5 text-sm text-noc-muted">
                                No daily data yet. SLA appears after the first rollup runs.
                            </p>
                        ) : (
                            <p className="mt-0.5 font-display text-display-sm text-status-up">
                                <SlidingNumber value={globalSla} decimals={2} suffix="%" />
                            </p>
                        )}
                    </div>
                    {unprobed.length > 0 && (
                        <p className="text-xs text-noc-muted">
                            {unprobed.length} targets have never been checked. Run the probe scheduler to
                            populate their status.
                        </p>
                    )}
                </Card>
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
                        <Link href="/targets" className="btn-primary mt-3">
                            Open the Targets page
                        </Link>
                    </Card>
                ) : (
                    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                        {targets.map((target) => (
                            <Link
                                key={target.id}
                                href={`/targets/${target.id}`}
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
                )}
            </section>
        </AppLayout>
    );
}

function SummaryCell({
    label,
    value,
    tone,
}: {
    label: string;
    value: number;
    tone?: 'up' | 'degraded' | 'down' | 'maintenance' | 'pending';
}) {
    const toneClass = {
        up: 'text-status-up',
        degraded: 'text-status-degraded',
        down: 'text-status-down',
        maintenance: 'text-status-maintenance',
        pending: 'text-noc-muted',
    }[tone ?? 'up'];

    return (
        <div className="bg-noc-surface px-4 py-3">
            <p className="field-label">{label}</p>
            <p className={`tabular mt-1 text-2xl font-semibold ${tone ? toneClass : 'text-noc-text'}`}>
                <SlidingNumber value={value} />
            </p>
        </div>
    );
}
