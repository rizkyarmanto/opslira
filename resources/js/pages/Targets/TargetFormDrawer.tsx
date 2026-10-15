import { useState } from 'react';
import { useForm, type UseFormRegister } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import SlideOverDrawer from '@/components/ui-layouts/SlideOverDrawer';
import TransitionPanel from '@/components/motion-primitives/TransitionPanel';
import Button from '@/components/shared/Button';
import { api, apiErrorMessage } from '@/lib/api';
import {
    configSchemaByType,
    dashboardGroupByTargetType,
    protocolByTargetType,
    targetFormSchema,
    type TargetFormValues,
} from '@/lib/schemas/target';
import type { ProbeResult, TargetType } from '@/types/monitoring';

const targetTypes: Array<{ value: TargetType; label: string }> = [
    { value: 'NETWORK_DEVICE', label: 'Network device (ICMP + SNMP)' },
    { value: 'WEB_APP', label: 'Website or application (HTTP health)' },
    { value: 'DOMAIN', label: 'Domain and TLS certificate (WHOIS)' },
    { value: 'DATABASE', label: 'Database (native ping)' },
    { value: 'MAIL_SERVER', label: 'Mail server (SMTP + DNSBL)' },
    { value: 'S3_STORAGE', label: 'S3 object storage (HeadBucket)' },
    { value: 'BESZEL_SERVER', label: 'Beszel server (API)' },
];

const environments = ['PRODUCTION', 'STAGING', 'DEVELOPMENT', 'INTERNAL'] as const;

/**
 * Add-target form. Test connection runs the real adapter and reports what came back,
 * including a failure, so the form tells the truth before the target is saved (R-26,
 * R-35).
 */
export default function TargetFormDrawer({
    open,
    onClose,
    onSaved,
}: {
    open: boolean;
    onClose: () => void;
    onSaved: () => void;
}) {
    const [testResult, setTestResult] = useState<ProbeResult | null>(null);
    const [testing, setTesting] = useState(false);
    const [saving, setSaving] = useState(false);
    const [saveError, setSaveError] = useState<string | null>(null);

    const {
        register,
        handleSubmit,
        watch,
        reset,
        formState: { errors },
    } = useForm<TargetFormValues>({
        resolver: zodResolver(targetFormSchema),
        defaultValues: {
            target_type: 'NETWORK_DEVICE',
            dashboard_group: 'NETWORK',
            environment: 'PRODUCTION',
            protocol_type: 'ICMP_SNMP',
            check_interval_seconds: 60,
            timeout_seconds: 5,
            retry_threshold: 3,
            degraded_threshold_ms: 1000,
            config_payload: {},
        },
    });

    const targetType = watch('target_type');

    const onSubmit = async (values: TargetFormValues) => {
        setSaving(true);
        setSaveError(null);
        try {
            await api.post('/targets', {
                ...values,
                dashboard_group: dashboardGroupByTargetType[values.target_type],
                protocol_type: protocolByTargetType[values.target_type],
            });
            reset();
            onSaved();
            onClose();
        } catch (error) {
            setSaveError(apiErrorMessage(error, 'Failed to save the target. Please review the form fields.'));
        } finally {
            setSaving(false);
        }
    };

    const onTestConnection = async () => {
        setTesting(true);
        setTestResult(null);
        try {
            const { data } = await api.post<ProbeResult>('/targets/test-connection', watch());
            setTestResult(data);
        } catch (error) {
            setTestResult({
                status: 'DOWN',
                responseTimeMs: null,
                errorMessage: apiErrorMessage(error, 'The connection test request could not be sent.'),
                metricsPayload: {},
            });
        } finally {
            setTesting(false);
        }
    };

    return (
        <SlideOverDrawer
            open={open}
            onClose={onClose}
            title="Add target"
            description="Fill in the endpoint and how it is checked. Test the connection before saving."
            footer={
                <div className="flex items-center justify-between gap-3">
                    <Button variant="ghost" loading={testing} onClick={onTestConnection}>
                        Test connection
                    </Button>
                    <Button loading={saving} onClick={handleSubmit(onSubmit)}>
                        Save
                    </Button>
                </div>
            }
        >
            <form className="space-y-4" onSubmit={handleSubmit(onSubmit)} noValidate>
                <Field label="Target name" error={errors.name?.message} htmlFor="target-name">
                    <input id="target-name" {...register('name')} className="input" placeholder="Example: Main office router" />
                </Field>

                <Field label="Target type" htmlFor="target-type">
                    <select id="target-type" {...register('target_type')} className="input">
                        {targetTypes.map((t) => (
                            <option key={t.value} value={t.value}>
                                {t.label}
                            </option>
                        ))}
                    </select>
                </Field>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <Field label="Environment" htmlFor="target-env">
                        <select id="target-env" {...register('environment')} className="input">
                            {environments.map((e) => (
                                <option key={e} value={e}>
                                    {e}
                                </option>
                            ))}
                        </select>
                    </Field>
                    <Field label="Check interval (seconds)" error={errors.check_interval_seconds?.message} htmlFor="target-interval">
                        <input id="target-interval" type="number" {...register('check_interval_seconds')} className="input tabular" />
                    </Field>
                </div>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <Field
                        label="Host or endpoint"
                        error={errors.host_or_endpoint?.message}
                        htmlFor="target-host"
                        className="sm:col-span-2"
                    >
                        <input id="target-host" {...register('host_or_endpoint')} className="input tabular" placeholder="192.168.1.1" />
                    </Field>
                    <Field label="Port" error={errors.port?.message} htmlFor="target-port">
                        <input id="target-port" type="number" {...register('port')} className="input tabular" placeholder="optional" />
                    </Field>
                </div>

                <TransitionPanel panelKey={targetType}>
                    <DynamicFields targetType={targetType} register={register} />
                </TransitionPanel>

                <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <Field label="Timeout (seconds)" htmlFor="target-timeout">
                        <input id="target-timeout" type="number" {...register('timeout_seconds')} className="input tabular" />
                    </Field>
                    <Field label="Consecutive failure threshold" htmlFor="target-retry">
                        <input id="target-retry" type="number" {...register('retry_threshold')} className="input tabular" />
                    </Field>
                    <Field label="Degraded threshold (ms)" htmlFor="target-degraded">
                        <input id="target-degraded" type="number" {...register('degraded_threshold_ms')} className="input tabular" />
                    </Field>
                </div>

                {testResult && (
                    <div
                        role="status"
                        className={`rounded-sm border px-3 py-2 text-xs ${
                            testResult.status === 'UP'
                                ? 'border-status-up/30 bg-status-up/10 text-status-up'
                                : testResult.status === 'DEGRADED'
                                  ? 'border-status-degraded/30 bg-status-degraded/10 text-status-degraded'
                                  : 'border-status-down/40 bg-status-down/10 text-status-down'
                        }`}
                    >
                        <p className="font-semibold">
                            Test result: {testResult.status}
                            {testResult.responseTimeMs !== null && `, ${testResult.responseTimeMs} ms`}
                        </p>
                        {testResult.errorMessage && <p className="mt-1">{testResult.errorMessage}</p>}
                    </div>
                )}

                {saveError && (
                    <p role="alert" className="rounded-sm border border-status-down/40 bg-status-down/10 px-3 py-2 text-xs text-status-down">
                        {saveError}
                    </p>
                )}
            </form>
        </SlideOverDrawer>
    );
}

function Field({
    label,
    error,
    htmlFor,
    className = '',
    children,
}: {
    label: string;
    error?: string;
    htmlFor: string;
    className?: string;
    children: React.ReactNode;
}) {
    return (
        <div className={className}>
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

function DynamicFields({
    targetType,
    register,
}: {
    targetType: TargetType;
    register: UseFormRegister<TargetFormValues>;
}) {
    const fields: Record<TargetType, Array<{ key: string; label: string; type?: string; placeholder?: string }>> = {
        NETWORK_DEVICE: [
            { key: 'sub_type', label: 'Device sub-type', placeholder: 'ROUTER, SWITCH, AP, or SERVER' },
            { key: 'icmp_count', label: 'ICMP packet count', type: 'number' },
        ],
        WEB_APP: [
            { key: 'method', label: 'HTTP method', placeholder: 'GET' },
            { key: 'expected_status', label: 'Expected status', type: 'number', placeholder: '200' },
            { key: 'expected_json_key', label: 'JSON key to check', placeholder: 'status' },
        ],
        DOMAIN: [
            { key: 'whois_server', label: 'WHOIS server', placeholder: 'whois.id' },
            { key: 'check_ssl_port', label: 'TLS port', type: 'number', placeholder: '443' },
            { key: 'warn_days_before', label: 'Warn before (days)', type: 'number', placeholder: '14' },
        ],
        DATABASE: [
            { key: 'db_driver', label: 'Driver', placeholder: 'pgsql, mysql, or sqlsrv' },
            { key: 'database_name', label: 'Database name' },
            { key: 'query', label: 'Test query', placeholder: 'SELECT 1' },
        ],
        MAIL_SERVER: [
            { key: 'check_dnsbl', label: 'Check DNSBL', placeholder: 'true or false' },
        ],
        S3_STORAGE: [
            { key: 'region', label: 'Region', placeholder: 'ap-southeast-1' },
            { key: 'bucket_name', label: 'Bucket name' },
        ],
        BESZEL_SERVER: [
            { key: 'beszel_system_id', label: 'Beszel system ID' },
            { key: 'warn_cpu_pct', label: 'CPU warning (%)', type: 'number', placeholder: '85' },
            { key: 'warn_disk_pct', label: 'Disk warning (%)', type: 'number', placeholder: '85' },
        ],
    };

    return (
        <div className="space-y-3 rounded-sm border border-noc-border bg-white/[0.02] p-4">
            <p className="field-label">Check method: {protocolByTargetType[targetType]}</p>
            {fields[targetType].map((field) => (
                <div key={field.key}>
                    <label htmlFor={`cfg-${field.key}`} className="field-label mb-1.5 block">
                        {field.label}
                    </label>
                    <input
                        id={`cfg-${field.key}`}
                        type={field.type ?? 'text'}
                        placeholder={field.placeholder}
                        {...register(`config_payload.${field.key}` as `config_payload.${string}`)}
                        className="input"
                    />
                </div>
            ))}
        </div>
    );
}

export { configSchemaByType };
