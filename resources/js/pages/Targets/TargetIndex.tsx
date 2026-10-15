import { useState } from 'react';
import { Head, Link, router } from '@inertiajs/react';
import { Plus, RefreshCw, Search } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Button from '@/components/shared/Button';
import StatusBadge from '@/components/shared/StatusBadge';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
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
export default function TargetIndex({ targets, filters }: Props) {
    const [drawerOpen, setDrawerOpen] = useState(false);
    const [group, setGroup] = useState(filters.dashboard_group ?? '');
    const [busyId, setBusyId] = useState<string | null>(null);

    const applyFilter = (value: string) => {
        setGroup(value);
        router.get('/targets', value ? { dashboard_group: value } : {}, { preserveState: true, replace: true });
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
        if (!confirm(`Disable target "${target.name}"? The target will no longer be checked.`)) return;
        setBusyId(target.id);
        try {
            await api.delete(`/targets/${target.id}`);
            router.reload({ only: ['targets'] });
        } finally {
            setBusyId(null);
        }
    };

    return (
        <AppLayout title="Targets">
            <Head title="Targets" />

            <div className="mb-5 flex flex-wrap items-center gap-3">
                <label className="flex items-center gap-2">
                    <span className="field-label">Group</span>
                    <select
                        value={group}
                        onChange={(e) => applyFilter(e.target.value)}
                        className="input w-auto"
                    >
                        <option value="">All groups</option>
                        <option value="NETWORK">Network</option>
                        <option value="APP_DOMAIN_DB">App, Domain &amp; DB</option>
                        <option value="MAIL_S3_BESZEL">Mail, S3 &amp; Beszel</option>
                    </select>
                </label>

                <Button
                    variant="ghost"
                    onClick={() => router.reload({ only: ['targets'] })}
                    aria-label="Reload target list"
                >
                    <RefreshCw size={15} aria-hidden />
                    Reload
                </Button>

                <Button className="ml-auto" onClick={() => setDrawerOpen(true)}>
                    <Plus size={15} aria-hidden />
                    Add target
                </Button>
            </div>

            {targets.length === 0 ? (
                <div className="rounded-md border border-dashed border-noc-border px-6 py-12 text-center">
                    <p className="text-sm text-noc-text">No targets match this filter yet.</p>
                    <p className="mx-auto mt-1 max-w-md text-xs text-noc-muted">
                        Add your first target to start monitoring a device, application, domain, database, mail
                        server, bucket, or Beszel server.
                    </p>
                    <Button className="mt-4" onClick={() => setDrawerOpen(true)}>
                        <Plus size={15} aria-hidden />
                        Add target
                    </Button>
                </div>
            ) : (
                <>
                    <div className="overflow-x-auto rounded-md border border-noc-border">
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
                                {targets.map((target) => (
                                    <tr key={target.id} className="hover:bg-white/[0.02]">
                                        <td className="px-4 py-3">
                                            <Link href={`/targets/${target.id}`} className="font-medium text-noc-text hover:underline">
                                                {target.name}
                                            </Link>
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
                                            <Button
                                                variant="ghost"
                                                loading={busyId === target.id}
                                                onClick={() => disableTarget(target)}
                                                className="!border-status-down/30 !px-3 !py-1.5 !text-xs !text-status-down hover:!bg-status-down/10"
                                            >
                                                Disable
                                            </Button>
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

            <TargetFormDrawer
                open={drawerOpen}
                onClose={() => setDrawerOpen(false)}
                onSaved={() => router.reload({ only: ['targets'] })}
            />
        </AppLayout>
    );
}
