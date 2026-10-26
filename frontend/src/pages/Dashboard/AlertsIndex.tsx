import { Link, useSearchParams } from 'react-router-dom';
import { Inbox } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import { Head } from '@/lib/head';
import { relativeTime } from '@/lib/time';
import type { Incident } from '@/types/monitoring';

type AlertFilter = 'open' | 'resolved' | 'all';

interface Props {
    incidents: Incident[];
    filter: AlertFilter;
    /** The list is being refetched on its polling interval. */
    refreshing: boolean;
    /** Time of the last successful load, for the "Updated" label. */
    lastUpdated: Date | null;
}

const filterOptions: Array<{ key: AlertFilter; label: string }> = [
    { key: 'open', label: 'Open' },
    { key: 'resolved', label: 'Resolved' },
    { key: 'all', label: 'All' },
];

const emptyCopy: Record<AlertFilter, string> = {
    open: 'No open alerts. New DOWN incidents will appear here.',
    resolved: 'No resolved alerts yet.',
    all: 'No alerts recorded yet.',
};

/**
 * Alerts: the incident list on its own screen. The filter tabs are the only
 * control and each one navigates, so the URL always says which list is on
 * screen. Rows reuse the NOC incident card so an alert reads the same here as
 * it does on the overview (DESIGN.md rhythm).
 */
export default function AlertsIndex({ incidents, filter, refreshing, lastUpdated }: Props) {
    const [, setSearchParams] = useSearchParams();

    const applyFilter = (next: AlertFilter) => {
        setSearchParams({ filter: next }, { replace: true });
    };

    return (
        <AppLayout title="Alerts">
            <Head title="Alerts" />

            <div className="mb-5 flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <p className="text-xs text-noc-muted">
                    {refreshing ? (
                        <ReloadIndicator label="Refreshing alerts" />
                    ) : lastUpdated ? (
                        `Updated ${lastUpdated.toLocaleTimeString('en-US')}`
                    ) : (
                        'Loading alerts...'
                    )}
                </p>
                <p className="text-xs text-noc-muted">Refreshes every 15 seconds while the tab is active.</p>
            </div>

            <div className="mb-4 flex flex-wrap items-center gap-2">
                <div role="group" aria-label="Filter alerts" className="flex items-center gap-1">
                    {filterOptions.map((option) => (
                        <button
                            key={option.key}
                            type="button"
                            aria-pressed={filter === option.key}
                            onClick={() => applyFilter(option.key)}
                            className={`rounded-xs px-2.5 py-1 text-xs transition-colors ${
                                filter === option.key
                                    ? 'bg-white/5 font-medium text-noc-text'
                                    : 'text-noc-muted hover:text-noc-text'
                            }`}
                        >
                            {option.label}
                        </button>
                    ))}
                </div>
                <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                    {incidents.length}
                </span>
            </div>

            {incidents.length === 0 ? (
                <Card className="flex items-center gap-3 px-4 py-6">
                    <Inbox size={16} className="text-noc-muted" aria-hidden />
                    <p className="text-sm text-noc-muted">{emptyCopy[filter]}</p>
                </Card>
            ) : (
                <div className="space-y-2">
                    {incidents.map((incident) => (
                        <Link
                            key={incident.id}
                            to={`/targets/${incident.target_id}`}
                            className="relative flex items-start gap-4 overflow-hidden rounded-md border border-noc-border bg-noc-surface px-4 py-3 pl-5 transition-colors hover:bg-white/[0.03]"
                        >
                            <span
                                aria-hidden
                                className={`absolute inset-y-0 left-0 w-[3px] ${
                                    incident.severity === 'CRITICAL' ? 'bg-status-down' : 'bg-status-degraded'
                                }`}
                            />
                            <span
                                className={`mt-0.5 shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold ${
                                    incident.severity === 'CRITICAL' ? 'status-down' : 'status-degraded'
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
                            <span className="tabular hidden shrink-0 text-right text-xs text-noc-muted sm:block">
                                <span className="block">{relativeTime(incident.started_at) ?? ''}</span>
                                {incident.resolved_at && (
                                    <span className="block">
                                        Resolved {relativeTime(incident.resolved_at) ?? ''}
                                    </span>
                                )}
                            </span>
                        </Link>
                    ))}
                </div>
            )}
        </AppLayout>
    );
}
