import { useEffect, useState } from 'react';
import SlideOverDrawer from '@/components/ui-layouts/SlideOverDrawer';
import Button from '@/components/shared/Button';
import { api, apiErrorMessage } from '@/lib/api';
import type { ReportSchedule } from '@/types/monitoring';

const periods = [
    { value: 'DAILY', label: 'Daily' },
    { value: 'WEEKLY', label: 'Weekly' },
    { value: 'MONTHLY', label: 'Monthly' },
    { value: 'CUSTOM_CRON', label: 'Custom cron' },
] as const;

const presets = [
    { cron: '0 8 * * *', label: 'Daily 08:00' },
    { cron: '0 8 * * 1', label: 'Weekly, Monday 08:00' },
    { cron: '0 8 1 * *', label: 'Monthly, 1st at 08:00' },
    { cron: '*/5 * * * *', label: 'Every 5 minutes' },
] as const;

const formats = ['HTML', 'PDF', 'XLSX'] as const;

interface FormState {
    name: string;
    period_type: (typeof periods)[number]['value'];
    cron_expression: string;
    recipients: string;
    report_formats: string[];
    send_only_on_issues: boolean;
    is_active: boolean;
}

const emptyState: FormState = {
    name: '',
    period_type: 'DAILY',
    cron_expression: '0 8 * * *',
    recipients: '',
    report_formats: ['HTML', 'PDF'],
    send_only_on_issues: false,
    is_active: true,
};

/**
 * Create or edit a report schedule. The cron field carries presets because most
 * schedules are one of four shapes, and the server still validates whatever is
 * submitted (R-26).
 */
export default function ScheduleFormDrawer({
    open,
    schedule,
    onClose,
    onSaved,
}: {
    open: boolean;
    schedule: ReportSchedule | null;
    onClose: () => void;
    onSaved: () => void;
}) {
    const [form, setForm] = useState<FormState>(emptyState);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [formatError, setFormatError] = useState<string | null>(null);

    useEffect(() => {
        if (!open) return;

        if (schedule) {
            setForm({
                name: schedule.name,
                period_type: schedule.period_type,
                cron_expression: schedule.cron_expression,
                recipients: schedule.recipients_to.join(', '),
                report_formats: schedule.report_formats,
                send_only_on_issues: schedule.send_only_on_issues,
                is_active: schedule.is_active,
            });
        } else {
            setForm(emptyState);
        }
        setError(null);
        setFormatError(null);
    }, [open, schedule]);

    const patch = (next: Partial<FormState>) => setForm((current) => ({ ...current, ...next }));

    const toggleFormat = (format: string) => {
        const selected = form.report_formats.includes(format)
            ? form.report_formats.filter((f) => f !== format)
            : [...form.report_formats, format];

        setFormatError(selected.length === 0 ? 'Choose at least one format.' : null);
        patch({ report_formats: selected });
    };

    const onSubmit = async () => {
        if (form.report_formats.length === 0) {
            setFormatError('Choose at least one format.');
            return;
        }

        setSaving(true);
        setError(null);

        const payload = {
            name: form.name.trim(),
            period_type: form.period_type,
            cron_expression: form.cron_expression.trim(),
            recipients_to: form.recipients
                .split(',')
                .map((entry) => entry.trim())
                .filter(Boolean),
            report_formats: form.report_formats,
            send_only_on_issues: form.send_only_on_issues,
            is_active: form.is_active,
        };

        try {
            if (schedule) {
                await api.put(`/reports/schedules/${schedule.id}`, payload);
            } else {
                await api.post('/reports/schedules', payload);
            }
            onSaved();
            onClose();
        } catch (caught) {
            // The drawer stays open so the operator can fix the field in place.
            setError(apiErrorMessage(caught, 'Failed to save the schedule.'));
        } finally {
            setSaving(false);
        }
    };

    return (
        <SlideOverDrawer
            open={open}
            onClose={onClose}
            title={schedule ? 'Edit schedule' : 'Add schedule'}
            description="The schedule drives when a report is generated and sent."
            footer={
                <div className="flex items-center justify-end gap-2">
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button onClick={() => void onSubmit()} loading={saving}>
                        {schedule ? 'Save changes' : 'Create schedule'}
                    </Button>
                </div>
            }
        >
            <div className="space-y-4">
                {error && (
                    <p role="alert" className="rounded-sm border border-status-down/40 bg-status-down/10 px-3 py-2 text-sm text-status-down">
                        {error}
                    </p>
                )}

                <Field label="Name" htmlFor="schedule-name">
                    <input
                        id="schedule-name"
                        className="input"
                        value={form.name}
                        onChange={(e) => patch({ name: e.target.value })}
                        placeholder="Daily NOC summary"
                        required
                    />
                </Field>

                <Field label="Period" htmlFor="schedule-period">
                    <select
                        id="schedule-period"
                        className="input"
                        value={form.period_type}
                        onChange={(e) => patch({ period_type: e.target.value as FormState['period_type'] })}
                    >
                        {periods.map((period) => (
                            <option key={period.value} value={period.value}>
                                {period.label}
                            </option>
                        ))}
                    </select>
                </Field>

                <Field label="Cron expression" htmlFor="schedule-cron">
                    <input
                        id="schedule-cron"
                        className="input tabular"
                        value={form.cron_expression}
                        onChange={(e) => patch({ cron_expression: e.target.value })}
                        placeholder="0 8 * * *"
                        required
                    />
                    <div className="mt-2 flex flex-wrap gap-1.5">
                        {presets.map((preset) => (
                            <button
                                key={preset.cron}
                                type="button"
                                onClick={() => patch({ cron_expression: preset.cron })}
                                className={`rounded-xs border px-2 py-1 text-[11px] transition-colors ${
                                    form.cron_expression === preset.cron
                                        ? 'border-noc-accent text-noc-accent'
                                        : 'border-noc-border text-noc-muted hover:text-noc-text'
                                }`}
                            >
                                {preset.label}
                            </button>
                        ))}
                    </div>
                    <p className="mt-1.5 text-[11px] text-noc-muted">
                        Five fields: minute hour day month weekday. Times use Asia/Jakarta.
                    </p>
                </Field>

                <Field label="Recipients" htmlFor="schedule-recipients">
                    <input
                        id="schedule-recipients"
                        className="input tabular"
                        value={form.recipients}
                        onChange={(e) => patch({ recipients: e.target.value })}
                        placeholder="noc@kantor.co.id, ops@kantor.co.id"
                    />
                    <p className="mt-1.5 text-[11px] text-noc-muted">
                        Comma separated. Leave empty to generate reports without sending them.
                    </p>
                </Field>

                <fieldset>
                    <legend className="field-label mb-1.5">Formats</legend>
                    <div className="flex flex-wrap gap-3">
                        {formats.map((format) => (
                            <label key={format} className="flex items-center gap-2 text-sm text-noc-text">
                                <input
                                    type="checkbox"
                                    checked={form.report_formats.includes(format)}
                                    onChange={() => toggleFormat(format)}
                                    className="h-4 w-4 accent-[#9fe870]"
                                />
                                {format}
                            </label>
                        ))}
                    </div>
                    {formatError && (
                        <p role="alert" className="mt-1 text-[11px] text-status-down">
                            {formatError}
                        </p>
                    )}
                </fieldset>

                <label className="flex items-center gap-2 text-sm text-noc-text">
                    <input
                        type="checkbox"
                        checked={form.send_only_on_issues}
                        onChange={(e) => patch({ send_only_on_issues: e.target.checked })}
                        className="h-4 w-4 accent-[#9fe870]"
                    />
                    Send only when there are incidents
                </label>

                <label className="flex items-center gap-2 text-sm text-noc-text">
                    <input
                        type="checkbox"
                        checked={form.is_active}
                        onChange={(e) => patch({ is_active: e.target.checked })}
                        className="h-4 w-4 accent-[#9fe870]"
                    />
                    Active
                </label>
            </div>
        </SlideOverDrawer>
    );
}

function Field({
    label,
    htmlFor,
    error,
    children,
}: {
    label: string;
    htmlFor: string;
    error?: string;
    children: React.ReactNode;
}) {
    return (
        <div>
            <label htmlFor={htmlFor} className="field-label mb-1.5 block">
                {label}
            </label>
            {children}
            {error && (
                <p role="alert" className="mt-1 text-[11px] text-status-down">
                    {error}
                </p>
            )}
        </div>
    );
}
