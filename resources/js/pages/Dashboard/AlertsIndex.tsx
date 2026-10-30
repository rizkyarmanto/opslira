import { useState } from 'react';
import { Head, Link, router } from '@inertiajs/react';
import { Inbox } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Button from '@/components/shared/Button';
import Card from '@/components/shared/Card';
import ConfirmModal from '@/components/shared/ConfirmModal';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import { api, apiErrorMessage } from '@/lib/api';
import { relativeTime } from '@/lib/time';
import { usePolling } from '@/lib/usePolling';
import type { Incident } from '@/types/monitoring';

type AlertFilter = 'open' | 'resolved' | 'all';

interface Props {
    incidents: Incident[];
    filter: AlertFilter;
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
 * navigation control and each one writes the URL, so the address bar always
 * says which list is on screen. Rows reuse the NOC incident card so an alert
 * reads the same here as it does on the overview (DESIGN.md rhythm).
 *
 * An open row carries a Resolve button. It sits above the row's link overlay,
 * so pressing it closes the incident without navigating to the target.
 */
export default function AlertsIndex({ incidents, filter }: Props) {
    const { lastUpdated, refreshing } = usePolling(['incidents']);
    const [resolveTarget, setResolveTarget] = useState<Incident | null>(null);
    const [resolving, setResolving] = useState(false);
    const [resolveError, setResolveError] = useState<string | null>(null);

    const applyFilter = (next: AlertFilter) => {
        router.get('/dashboard/alerts', { filter: next }, { preserveState: true, replace: true });
    };

    const openResolve = (incident: Incident) => {
        setResolveError(null);
        setResolveTarget(incident);
    };

    const confirmResolve = async () => {
        if (!resolveTarget) return;
        setResolving(true);
        try {
            await api.post(`/incidents/${resolveTarget.id}/resolve`);
            setResolveTarget(null);
            router.reload({ only: ['incidents'] });
        } catch (error) {
            setResolveError(apiErrorMessage(error, 'Could not resolve this alert.'));
        } finally {
            setResolving(false);
        }
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
                        <div
                            key={incident.id}
                            className="relative flex items-start gap-4 overflow-hidden rounded-md border border-noc-border bg-noc-surface py-3 pl-5 pr-4 transition-colors hover:bg-white/[0.03]"
                        >
                            <Link
                                href={`/targets/${incident.target_id}`}
                                aria-label={`Open ${incident.target_name ?? 'target'}`}
                                className="absolute inset-0"
                            />
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
                            {incident.status !== 'RESOLVED' && (
                                <div className="relative z-10 shrink-0 self-center">
                                    <Button
                                        variant="ghost"
                                        className="!px-3 !py-1.5 !text-xs"
                                        loading={resolving && resolveTarget?.id === incident.id}
                                        onClick={() => openResolve(incident)}
                                    >
                                        Resolve
                                    </Button>
                                </div>
                            )}
                        </div>
                    ))}
                </div>
            )}

            <ConfirmModal
                tone="info"
                open={resolveTarget !== null}
                title="Resolve alert"
                message={`Mark the alert for "${resolveTarget?.target_name ?? 'this target'}" as resolved? The target keeps its probe status.`}
                confirmLabel="Resolve"
                pending={resolving}
                error={resolveError ?? undefined}
                onConfirm={confirmResolve}
                onClose={() => setResolveTarget(null)}
            />
        </AppLayout>
    );
}
