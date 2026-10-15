import { useState } from 'react';
import { Head, router } from '@inertiajs/react';
import { CalendarClock, FileText, Play } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import Button from '@/components/shared/Button';
import ToggleSwitch from '@/components/shared/ToggleSwitch';
import { api, apiErrorMessage } from '@/lib/api';
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
    const [running, setRunning] = useState<string | null>(null);
    const [toggling, setToggling] = useState<string | null>(null);
    const [notice, setNotice] = useState<{ tone: 'ok' | 'info' | 'error'; text: string } | null>(null);

    const toggleActive = async (schedule: ReportSchedule) => {
        setToggling(schedule.id);
        setNotice(null);
        try {
            await api.put(`/reports/schedules/${schedule.id}`, { is_active: !schedule.is_active });
            router.reload({ only: ['schedules'] });
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
            const { data } = await api.post<{ ok: boolean; outcome: string; message: string }>(
                `/reports/schedules/${schedule.id}/run-now`,
            );
            setNotice({
                tone: data.outcome === 'created' ? 'ok' : 'info',
                text: data.outcome === 'created' ? `Report "${schedule.name}" was created and sent.` : data.message,
            });
            router.reload({ only: ['schedules'] });
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
                    Reports are generated automatically on a cron schedule, then sent to registered recipients.
                </p>
                <a href="/reports/email-logs" className="text-xs text-noc-accent hover:underline">
                    View report and email history
                </a>
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
                        Report schedules are created through the seeder or the schedule endpoint. Once a schedule
                        exists, its card appears here.
                    </p>
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
                                <span className="tabular text-[10px] text-noc-muted">
                                    {schedule.next_run_at
                                        ? `Next ${formatDateTime(schedule.next_run_at)}`
                                        : 'Next run not calculated yet'}
                                </span>
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
                        </Card>
                    ))}
                </div>
            )}
        </AppLayout>
    );
}
