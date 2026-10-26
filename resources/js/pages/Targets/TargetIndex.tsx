import { useState } from 'react';
import { Head, Link, router } from '@inertiajs/react';
import { Plus, RefreshCw, Search } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Button from '@/components/shared/Button';
import StatusBadge from '@/components/shared/StatusBadge';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import ConfirmModal from '@/components/shared/ConfirmModal';
import ToggleSwitch from '@/components/shared/ToggleSwitch';
import TargetFormDrawer from '@/pages/Targets/TargetFormDrawer';
import { api } from '@/lib/api';
import { relativeTime } from '@/lib/time';
import type { MonitoringTarget } from '@/types/monitoring';

interface Props {
    targets: MonitoringTarget[];
    filters: Record<string, string>;
}

/**
 * Target management. The columns are the ones that decide what the operator does
 * next: what it is, where it is, whether it is up, and the action on it. The name is
 * the link into the detail page (R-20, R-26).
 */
export default function TargetIndex({ targets, filters: initialFilters }: Props) {
    const [drawerOpen, setDrawerOpen] = useState(false);
    const [editingTarget, setEditingTarget] = useState<MonitoringTarget | null>(null);
    const [busyId, setBusyId] = useState<string | null>(null);
    const [reloading, setReloading] = useState(false);
    const [confirmTarget, setConfirmTarget] = useState<MonitoringTarget | null>(null);
    const [filters, setFilters] = useState({
        status: initialFilters.status ?? '',
        state: initialFilters.state ?? 'active',
        // The server hides maintenance rows unless asked, so the checkbox says the same.
        maintenance: initialFilters.maintenance ?? 'no',
    });
    const [search, setSearch] = useState('');

    const hasActiveFilter =
        search !== '' ||
        filters.status !== '' ||
        filters.maintenance !== 'no' ||
        filters.state !== 'active';

    const applyFilters = (next: Partial<typeof filters>) => {
        const merged = { ...filters, ...next };
        setFilters(merged);

        const params: Record<string, string> = {};
        for (const [key, value] of Object.entries(merged)) {
            if (value) params[key] = value;
        }
        router.get('/targets', params, { preserveState: true, replace: true });
    };

    const clearFilters = () => {
        setSearch('');
        applyFilters({ status: '', state: 'active', maintenance: 'no' });
    };

    const query = search.trim().toLowerCase();
    const displayedTargets =
        query === ''
            ? targets
            : targets.filter((target) =>
                  `${target.name} ${target.host_or_endpoint} ${target.target_type}`.toLowerCase().includes(query),
              );

    const reloadTargets = () => {
        router.reload({
            only: ['targets'],
            onStart: () => setReloading(true),
            onFinish: () => setReloading(false),
        });
    };

    const toggleMaintenance = async (target: MonitoringTarget) => {
        setBusyId(target.id);
        try {
            await api.post(`/targets/${target.id}/toggle-maintenance`);
            router.reload({ only: ['targets'] });
        } finally {
            setBusyId(null);
        }
    };

    const disableTarget = async (target: MonitoringTarget) => {
        setBusyId(target.id);
        try {
            await api.delete(`/targets/${target.id}`);
            setConfirmTarget(null);
            router.reload({ only: ['targets'] });
        } finally {
            setBusyId(null);
        }
    };

    const enableTarget = async (target: MonitoringTarget) => {
        setBusyId(target.id);
        try {
            await api.post(`/targets/${target.id}/enable`);
            router.reload({ only: ['targets'] });
        } finally {
            setBusyId(null);
        }
    };
    return (
        <AppLayout title="Targets">
            <Head title="Targets" />

            <div className="mb-5 flex flex-wrap items-center gap-3">
                <input
                    type="search"
                    value={search}
                    onChange={(e) => setSearch(e.target.value)}
                    placeholder="Filter by name, host, or type..."
                    aria-label="Filter targets"
                    className="input w-56"
                />

                <div role="group" aria-label="Status" className="flex items-center gap-1">
                    {[
                        { label: 'All', value: '' },
                        { label: 'UP', value: 'UP' },
                        { label: 'DEGRADED', value: 'DEGRADED' },
                        { label: 'DOWN', value: 'DOWN' },
                        { label: 'MAINTENANCE', value: 'MAINTENANCE' },
                    ].map((option) => (
                        <button
                            key={option.label}
                            type="button"
                            aria-pressed={filters.status === option.value}
                            onClick={() => applyFilters({ status: option.value })}
                            className={`rounded-sm px-2.5 py-1.5 text-xs tabular ${filters.status === option.value ? 'bg-white/10 text-noc-text' : 'text-noc-muted'}`}
                        >
                            {option.label}
                        </button>
                    ))}
                </div>

                <div role="group" aria-label="State" className="flex items-center gap-1">
                    {[
                        { label: 'Active', value: 'active' },
                        { label: 'Disabled', value: 'disabled' },
                        { label: 'All', value: 'all' },
                    ].map((option) => (
                        <button
                            key={option.label}
                            type="button"
                            aria-pressed={filters.state === option.value}
                            onClick={() => applyFilters({ state: option.value })}
                            className={`rounded-sm px-2.5 py-1.5 text-xs tabular ${filters.state === option.value ? 'bg-white/10 text-noc-text' : 'text-noc-muted'}`}
                        >
                            {option.label}
                        </button>
                    ))}
                </div>

                <label className="flex items-center gap-2 text-xs text-noc-muted">
                    <input
                        type="checkbox"
                        checked={filters.maintenance === 'yes'}
                        onChange={(e) => applyFilters({ maintenance: e.target.checked ? 'yes' : 'no' })}
                    />
                    In maintenance
                </label>

                {hasActiveFilter && (
                    <Button variant="ghost" onClick={clearFilters} className="!px-3 !py-1.5 !text-xs">
                        Clear filters
                    </Button>
                )}

                <span className="tabular text-xs text-noc-muted">{displayedTargets.length} targets</span>

                <Button
                    variant="ghost"
                    onClick={reloadTargets}
                    aria-label="Reload target list"
                >
                    {reloading ? (
                        <ReloadIndicator label="Reloading" />
                    ) : (
                        <RefreshCw size={15} aria-hidden />
                    )}
                    Reload
                </Button>

                <Button className="ml-auto" onClick={() => { setEditingTarget(null); setDrawerOpen(true); }}>
                    <Plus size={15} aria-hidden />
                    Add target
                </Button>
            </div>

            {displayedTargets.length === 0 ? (
                <div className="rounded-md border border-dashed border-noc-border px-6 py-12 text-center">
                    <p className="text-sm text-noc-text">
                        {hasActiveFilter ? 'No targets match these filters.' : 'No targets match this filter yet.'}
                    </p>
                    {hasActiveFilter ? (
                        <Button className="mt-4" variant="ghost" onClick={clearFilters}>
                            Clear filters
                        </Button>
                    ) : (
                        <>
                            <p className="mx-auto mt-1 max-w-md text-xs text-noc-muted">
                                Add your first target to start monitoring a device, application, domain, database, mail
                                server, bucket, or Beszel server.
                            </p>
                            <Button className="mt-4" onClick={() => setDrawerOpen(true)}>
                                <Plus size={15} aria-hidden />
                                Add target
                            </Button>
                        </>
                    )}
                </div>
            ) : (
                <>
                    {reloading && (
                        <p className="mb-2 text-xs text-noc-muted">
                            <ReloadIndicator label="Refreshing target list" />
                        </p>
                    )}
                    <div
                        className={`overflow-x-auto rounded-md border border-noc-border${
                            reloading ? ' opacity-60 pointer-events-none' : ''
                        }`}
                        aria-busy={reloading}
                    >
                        <table className="w-full min-w-[760px] text-left text-sm">
                            <thead className="bg-white/[0.03]">
                                <tr>
                                    <th scope="col" className="field-label px-4 py-3">Name</th>
                                    <th scope="col" className="field-label px-4 py-3">Endpoint</th>
                                    <th scope="col" className="field-label px-4 py-3">Check method</th>
                                    <th scope="col" className="field-label px-4 py-3">Status</th>
                                    <th scope="col" className="field-label px-4 py-3">Last check</th>
                                    <th scope="col" className="field-label px-4 py-3">Maintenance</th>
                                    <th scope="col" className="field-label px-4 py-3 text-right">Action</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-noc-border">
                                {displayedTargets.map((target) => (
                                    <tr key={target.id} className="hover:bg-white/[0.02]">
                                        <td className="px-4 py-3">
                                            <Link
                                                href={`/targets/${target.id}`}
                                                className={`font-medium hover:underline ${
                                                    target.is_active ? 'text-noc-text' : 'text-noc-muted'
                                                }`}
                                            >
                                                {target.name}
                                            </Link>
                                            {!target.is_active && (
                                                <span className="ml-2 rounded-xs border border-noc-border px-1.5 py-0.5 text-[10px] font-semibold text-noc-muted">
                                                    DISABLED
                                                </span>
                                            )}
                                            <p className="text-[11px] text-noc-muted">{target.environment}</p>
                                        </td>
                                        <td className="tabular px-4 py-3 text-noc-muted">
                                            {target.host_or_endpoint}
                                            {target.port ? `:${target.port}` : ''}
                                        </td>
                                        <td className="px-4 py-3">
                                            <ProtocolBadge protocolType={target.protocol_type} />
                                        </td>
                                        <td className="px-4 py-3">
                                            <StatusBadge status={target.current_status} />
                                        </td>
                                        <td className="px-4 py-3 text-xs text-noc-muted">
                                            {relativeTime(target.last_checked_at) ?? 'Not checked yet'}
                                        </td>
                                        <td className="px-4 py-3">
                                            <ToggleSwitch
                                                tone="maintenance"
                                                checked={target.is_maintenance}
                                                disabled={busyId === target.id}
                                                onChange={() => toggleMaintenance(target)}
                                                label={`Maintenance for ${target.name}`}
                                            />
                                        </td>
                                        <td className="px-4 py-3 text-right">
                                            <div className="inline-flex gap-2">
                                                <Button
                                                    variant="ghost"
                                                    onClick={() => setEditingTarget(target)}
                                                    className="!px-3 !py-1.5 !text-xs"
                                                >
                                                    Edit
                                                </Button>
                                            {target.is_active ? (
                                                <Button
                                                    variant="ghost"
                                                    loading={busyId === target.id}
                                                    onClick={() => setConfirmTarget(target)}
                                                    className="!border-status-down/30 !px-3 !py-1.5 !text-xs !text-status-down hover:!bg-status-down/10"
                                                >
                                                    Disable
                                                </Button>
                                            ) : (
                                                <Button
                                                    variant="ghost"
                                                    loading={busyId === target.id}
                                                    onClick={() => enableTarget(target)}
                                                    className="!px-3 !py-1.5 !text-xs"
                                                >
                                                    Enable
                                                </Button>
                                            )}
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>

                    <p className="mt-3 flex items-center gap-1.5 text-xs text-noc-muted">
                        <Search size={12} aria-hidden />
                        Use Ctrl K to search for a target quickly.
                    </p>
                </>
            )}

            <ConfirmModal
                tone="danger"
                open={confirmTarget !== null}
                title={`Disable target "${confirmTarget?.name ?? ''}"?`}
                message="The target will no longer be checked."
                confirmLabel="Disable"
                pending={busyId === confirmTarget?.id}
                onConfirm={() => confirmTarget && disableTarget(confirmTarget)}
                onClose={() => setConfirmTarget(null)}
            />

            <TargetFormDrawer
                open={drawerOpen || editingTarget !== null}
                onClose={() => { setDrawerOpen(false); setEditingTarget(null); }}
                onSaved={reloadTargets}
                editing={editingTarget}
            />
        </AppLayout>
    );
}
