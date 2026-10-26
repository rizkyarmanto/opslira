import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { CalendarClock, FileText, Pencil, Play, Plus, Trash2 } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import Button from '@/components/shared/Button';
import ConfirmModal from '@/components/shared/ConfirmModal';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import ToggleSwitch from '@/components/shared/ToggleSwitch';
import ScheduleFormDrawer from '@/pages/Reports/ScheduleFormDrawer';
import { api, apiErrorMessage } from '@/lib/api';
import { Head } from '@/lib/head';
import { queryKeys } from '@/lib/query-keys';
import { formatDateTime } from '@/lib/time';
import type { ReportSchedule } from '@/types/monitoring';

interface Props {
    schedules: ReportSchedule[];
}

/**
 * Report schedules. Each card shows when it runs next and what it produces, and both
 * controls report their outcome: the toggle reloads the list, and Run now tells the
 * user the job was queued or why it failed (R-26, R-35).
 */
export default function ScheduleManager({ schedules }: Props) {
    const queryClient = useQueryClient();
    const [running, setRunning] = useState<string | null>(null);
    const [toggling, setToggling] = useState<string | null>(null);
    const [reloading, setReloading] = useState(false);
    const [deleting, setDeleting] = useState<string | null>(null);
    const [confirmSchedule, setConfirmSchedule] = useState<ReportSchedule | null>(null);
    const [drawerOpen, setDrawerOpen] = useState(false);
    const [editing, setEditing] = useState<ReportSchedule | null>(null);
    const [notice, setNotice] = useState<{ tone: 'ok' | 'info' | 'error'; text: string } | null>(null);

    const reloadSchedules = async () => {
        setReloading(true);
        try {
            await queryClient.invalidateQueries({ queryKey: queryKeys.reports.all });
        } finally {
            setReloading(false);
        }
    };

    const openCreate = () => {
        setEditing(null);
        setDrawerOpen(true);
    };

    const openEdit = (schedule: ReportSchedule) => {
        setEditing(schedule);
        setDrawerOpen(true);
    };

    const removeSchedule = async () => {
        if (!confirmSchedule) {
            return;
        }
        const schedule = confirmSchedule;
        setDeleting(schedule.id);
        setNotice(null);
        try {
            await api.delete(`/reports/schedules/${schedule.id}`);
            setNotice({ tone: 'ok', text: `Schedule "${schedule.name}" was deleted.` });
            setConfirmSchedule(null);
            reloadSchedules();
        } catch (error) {
            setNotice({ tone: 'error', text: apiErrorMessage(error, 'Failed to delete the schedule.') });
        } finally {
            setDeleting(null);
        }
    };

    const toggleActive = async (schedule: ReportSchedule) => {
        setToggling(schedule.id);
        setNotice(null);
        try {
            await api.put(`/reports/schedules/${schedule.id}`, { is_active: !schedule.is_active });
            reloadSchedules();
        } catch (error) {
            setNotice({ tone: 'error', text: apiErrorMessage(error, 'Failed to change the schedule status.') });
        } finally {
            setToggling(null);
        }
    };

    const runNow = async (schedule: ReportSchedule) => {
        setRunning(schedule.id);
        setNotice(null);
        try {
            const { data } = await api.post<{
                ok: boolean;
                outcome: string;
                message: string;
                sent?: number | null;
                failed?: number | null;
            }>(`/reports/schedules/${schedule.id}/run-now`);
            const failed = data.failed ?? 0;
            setNotice({
                tone: !data.ok ? 'error' : data.outcome !== 'created' || failed > 0 ? 'info' : 'ok',
                text: data.message,
            });
            reloadSchedules();
        } catch (error) {
            setNotice({ tone: 'error', text: apiErrorMessage(error, 'Failed to run this schedule.') });
        } finally {
            setRunning(null);
        }
    };

    return (
        <AppLayout title="Schedules & Reports">
            <Head title="Schedules & Reports" />

            <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
                <p className="text-xs text-noc-muted">
                    {reloading ? (
                        <ReloadIndicator label="Loading schedules" />
                    ) : (
                        'Reports are generated automatically on a cron schedule, then sent to registered recipients.'
                    )}
                </p>
                <div className="flex items-center gap-4">
                    <Link to="/reports/email-logs" className="text-xs text-noc-accent hover:underline">
                        View report and email history
                    </Link>
                    <Button onClick={openCreate}>
                        <Plus size={15} aria-hidden />
                        Add schedule
                    </Button>
                </div>
            </div>

            {notice && (
                <p
                    role={notice.tone === 'error' ? 'alert' : 'status'}
                    className={`mb-4 rounded-sm border px-3 py-2 text-sm ${
                        notice.tone === 'error'
                            ? 'border-status-down/40 bg-status-down/10 text-status-down'
                            : notice.tone === 'info'
                              ? 'border-noc-border bg-white/5 text-noc-text'
                              : 'border-status-up/30 bg-status-up/10 text-status-up'
                    }`}
                >
                    {notice.text}
                </p>
            )}

            {schedules.length === 0 ? (
                <div className="rounded-md border border-dashed border-noc-border px-6 py-12 text-center">
                    <CalendarClock size={20} className="mx-auto text-noc-muted" aria-hidden />
                    <p className="mt-3 text-sm text-noc-text">No report schedules yet.</p>
                    <p className="mx-auto mt-1 max-w-md text-xs text-noc-muted">
                        Add the first schedule to start generating reports.
                    </p>
                    <Button className="mt-4" onClick={openCreate}>
                        <Plus size={15} aria-hidden />
                        Add schedule
                    </Button>
                </div>
            ) : (
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
                    {schedules.map((schedule) => (
                        <Card key={schedule.id} className="flex flex-col p-5">
                            <div className="mb-3 flex items-start justify-between gap-3">
                                <div className="flex min-w-0 items-start gap-2">
                                    <FileText size={16} className="mt-0.5 shrink-0 text-noc-muted" aria-hidden />
                                    <div className="min-w-0">
                                        <p className="truncate text-sm font-medium text-noc-text">{schedule.name}</p>
                                        <p className="tabular text-xs text-noc-muted">
                                            {schedule.period_type} · {schedule.cron_expression} ({schedule.timezone})
                                        </p>
                                    </div>
                                </div>
                                <ToggleSwitch
                                    checked={schedule.is_active}
                                    disabled={toggling === schedule.id}
                                    onChange={() => toggleActive(schedule)}
                                    label={`Enable schedule ${schedule.name}`}
                                />
                            </div>

                            <div className="mb-3 flex flex-wrap gap-1.5">
                                {schedule.report_formats.map((format) => (
                                    <span
                                        key={format}
                                        className="rounded-xs border border-noc-border px-2 py-0.5 text-[10px] font-semibold text-noc-muted"
                                    >
                                        {format}
                                    </span>
                                ))}
                            </div>

                            <div className="mb-4">
                                <p className="field-label mb-1.5">Recipients</p>
                                {schedule.recipients_to.length === 0 ? (
                                    <p className="text-[11px] text-noc-muted">
                                        No recipients yet. Reports will be generated but not sent.
                                    </p>
                                ) : (
                                    <div className="flex flex-wrap gap-1.5">
                                        {schedule.recipients_to.map((email) => (
                                            <span
                                                key={email}
                                                className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[10px] text-noc-text"
                                            >
                                                {email}
                                            </span>
                                        ))}
                                    </div>
                                )}
                            </div>

                            <div className="mt-auto flex items-center justify-between gap-2 border-t border-noc-border pt-3">
                                {running === schedule.id ? (
                                    <ReloadIndicator label="Running" />
                                ) : (
                                    <span className="tabular text-[10px] text-noc-muted">
                                        {schedule.next_run_at
                                            ? `Next ${formatDateTime(schedule.next_run_at)}`
                                            : 'Next run not calculated yet'}
                                    </span>
                                )}
                                <div className="flex items-center gap-1.5">
                                    <Button
                                        variant="ghost"
                                        onClick={() => openEdit(schedule)}
                                        className="!px-3 !py-1.5 !text-xs"
                                    >
                                        <Pencil size={12} aria-hidden />
                                        Edit
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        loading={deleting === schedule.id}
                                        onClick={() => setConfirmSchedule(schedule)}
                                        className="!border-status-down/30 !px-3 !py-1.5 !text-xs !text-status-down hover:!bg-status-down/10"
                                    >
                                        <Trash2 size={12} aria-hidden />
                                        Delete
                                    </Button>
                                    <Button
                                        variant="ghost"
                                        loading={running === schedule.id}
                                        onClick={() => runNow(schedule)}
                                        className="!px-3 !py-1.5 !text-xs"
                                    >
                                        <Play size={12} aria-hidden />
                                        Run now
                                    </Button>
                                </div>
                            </div>
                        </Card>
                    ))}
                </div>
            )}

            <ConfirmModal
                open={confirmSchedule !== null}
                title={`Delete schedule "${confirmSchedule?.name ?? ''}"?`}
                message="Reports already generated stay in the history."
                confirmLabel="Delete"
                pending={deleting === confirmSchedule?.id}
                onConfirm={() => void removeSchedule()}
                onClose={() => setConfirmSchedule(null)}
            />

            <ScheduleFormDrawer
                open={drawerOpen}
                schedule={editing}
                onClose={() => setDrawerOpen(false)}
                onSaved={reloadSchedules}
            />
        </AppLayout>
    );
}
