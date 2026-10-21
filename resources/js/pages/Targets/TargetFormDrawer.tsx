import { useEffect, useState } from 'react';
import { useForm, type FieldErrors, type UseFormRegister, type UseFormSetValue, type UseFormWatch } from 'react-hook-form';
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
    { value: 'INTERNET', label: 'Internet host (ICMP ping)' },
    { value: 'WEB_APP', label: 'Website or application (HTTP health)' },
    { value: 'DOMAIN', label: 'Domain and TLS certificate (WHOIS)' },
    { value: 'DATABASE', label: 'Database (native ping)' },
    { value: 'MAIL_SERVER', label: 'Mail server (SMTP + DNSBL)' },
    { value: 'S3_STORAGE', label: 'S3 object storage (HeadBucket)' },
    { value: 'BESZEL_SERVER', label: 'Beszel server (API)' },
];

const environments = ['PRODUCTION', 'STAGING', 'DEVELOPMENT', 'INTERNAL'] as const;

/** The endpoint field keeps the host_or_endpoint name; only label and hint change per type. */
const hostFieldByType: Record<TargetType, { label: string; placeholder: string }> = {
    NETWORK_DEVICE: { label: 'Host', placeholder: '192.168.1.1' },
    INTERNET: { label: 'Host', placeholder: '8.8.8.8 or example.com' },
    WEB_APP: { label: 'Website URL', placeholder: 'https://portal.kantor.co.id/health' },
    DOMAIN: { label: 'Domain Name', placeholder: 'kantor.co.id' },
    DATABASE: { label: 'Database URL', placeholder: 'db-primary.example.internal' },
    MAIL_SERVER: { label: 'URL SMTP', placeholder: 'smtp.kantor.co.id' },
    S3_STORAGE: { label: 'S3 URL', placeholder: 'https://s3.ap-southeast-1.amazonaws.com' },
    BESZEL_SERVER: { label: 'Beszel URL', placeholder: 'http://beszel.kantor.co.id:8090' },
};

/**
 * WHOIS servers verified live this session: port 43 query answered AND the reply
 * matched the adapter's expiry regexes. uk/de/jp/eu answer but their formats are
 * unparsed, so they are left out until whoisExpiry learns their patterns.
 */
const whoisServerOptions: Array<{ label: string; value: string }> = [
    { label: 'Auto (derive from domain)', value: '' },
    { label: '.id / .co.id / .or.id / .go.id / .ac.id / .web.id (PANDI)', value: 'whois.id' },
    { label: '.com / .net (Verisign)', value: 'whois.verisign-grs.com' },
    { label: '.org (PIR)', value: 'whois.publicinterestregistry.org' },
    { label: '.io', value: 'whois.nic.io' },
    { label: '.co', value: 'whois.registry.co' },
    { label: '.xyz', value: 'whois.nic.xyz' },
    { label: '.me', value: 'whois.nic.me' },
    { label: '.biz', value: 'whois.nic.biz' },
];

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
        setValue,
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
            credentials: { auth_type: 'NONE', payload: {} },
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
            // The form has no protocol input; derive it from the target type the
            // same way onSubmit does, otherwise a WEB_APP test would run the
            // ICMP adapter and report "ping failed".
            const values = watch();
            const { data } = await api.post<ProbeResult>('/targets/test-connection', {
                ...values,
                dashboard_group: dashboardGroupByTargetType[values.target_type],
                protocol_type: protocolByTargetType[values.target_type],
            });
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
                    <select
                        id="target-type"
                        {...register('target_type', {
                            // Each type has its own endpoint values; without this the
                            // previous type's config_payload keys (region, bucket, ...),
                            // credentials and host/port would survive the panel remount
                            // and ride along into the next request.
                            onChange: () => {
                                setValue('config_payload', {});
                                setValue('host_or_endpoint', '');
                                setValue('port', null);
                            },
                        })}
                        className="input"
                    >
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

                <TransitionPanel panelKey={targetType}>
                    <div className="space-y-3">
                        <DynamicFields targetType={targetType} register={register} errors={errors} />
                        <CredentialsSection targetType={targetType} register={register} setValue={setValue} watch={watch} />
                    </div>
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
    errors,
}: {
    targetType: TargetType;
    register: UseFormRegister<TargetFormValues>;
    errors: FieldErrors<TargetFormValues>;
}) {
    const fields: Record<
        TargetType,
        Array<{ key: string; label: string; type?: string; placeholder?: string; options?: Array<{ label: string; value: string }> }>
    > = {
        NETWORK_DEVICE: [
            {
                key: 'sub_type',
                label: 'Device sub-type',
                type: 'select',
                options: [
                    { label: 'ROUTER', value: 'ROUTER' },
                    { label: 'SWITCH', value: 'SWITCH' },
                    { label: 'AP', value: 'AP' },
                    { label: 'SERVER', value: 'SERVER' },
                ],
            },
            { key: 'icmp_count', label: 'ICMP packet count', type: 'number' },
        ],
        INTERNET: [{ key: 'icmp_count', label: 'ICMP packet count', type: 'number' }],
        WEB_APP: [
            { key: 'method', label: 'HTTP method', placeholder: 'GET' },
            { key: 'expected_status', label: 'Expected status', type: 'number', placeholder: '200' },
        ],
        DOMAIN: [
            { key: 'check_ssl_port', label: 'TLS port', type: 'number', placeholder: '443' },
            { key: 'warn_days_before', label: 'Warn before (days)', type: 'number', placeholder: '14' },
        ],
        DATABASE: [
            { key: 'db_driver', label: 'Driver', placeholder: 'pgsql, mysql, or sqlsrv' },
            { key: 'database_name', label: 'Database name' },
            { key: 'query', label: 'Test query', placeholder: 'SELECT 1' },
            { key: 'check_replication_lag', label: 'Check replication lag', type: 'checkbox' },
        ],
        MAIL_SERVER: [
            {
                key: 'mail_check_mode',
                label: 'Check via',
                type: 'select',
                options: [
                    { label: 'SMTP', value: 'smtp' },
                    { label: 'IMAP', value: 'imap' },
                ],
            },
            { key: 'check_dnsbl', label: 'Check DNSBL', placeholder: 'true or false' },
        ],
        S3_STORAGE: [
            {
                key: 's3_provider',
                label: 'S3 provider',
                type: 'select',
                options: [
                    { label: 'AWS S3', value: 'aws' },
                    { label: 'S3 Compatible (MinIO, RustFS, Ceph)', value: 'compatible' },
                ],
            },
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
            <Field label={hostFieldByType[targetType].label} error={errors.host_or_endpoint?.message} htmlFor="target-host">
                <input
                    id="target-host"
                    {...register('host_or_endpoint')}
                    className="input tabular"
                    placeholder={hostFieldByType[targetType].placeholder}
                />
            </Field>
            <Field label="Port" error={errors.port?.message} htmlFor="target-port">
                <input id="target-port" type="number" {...register('port')} className="input tabular" placeholder="optional" />
            </Field>
            {targetType === 'DOMAIN' && (
                <Field label="WHOIS server" htmlFor="cfg-whois_server">
                    <select id="cfg-whois_server" {...register('config_payload.whois_server')} className="input">
                        {whoisServerOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                                {option.label}
                            </option>
                        ))}
                    </select>
                </Field>
            )}
            {fields[targetType].map((field) =>
                field.options ? (
                    <div key={field.key}>
                        <label htmlFor={`cfg-${field.key}`} className="field-label mb-1.5 block">
                            {field.label}
                        </label>
                        <select id={`cfg-${field.key}`} {...register(`config_payload.${field.key}` as `config_payload.${string}`)} className="input">
                            {field.options.map((option) => (
                                <option key={option.value} value={option.value}>
                                    {option.label}
                                </option>
                            ))}
                        </select>
                    </div>
                ) : field.type === 'checkbox' ? (
                    <div key={field.key} className="flex items-center gap-2">
                        <input
                            id={`cfg-${field.key}`}
                            type="checkbox"
                            {...register(`config_payload.${field.key}` as `config_payload.${string}`)}
                            className="size-4 accent-noc-accent"
                        />
                        <label htmlFor={`cfg-${field.key}`} className="field-label">
                            {field.label}
                        </label>
                    </div>
                ) : (
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
                ),
            )}
        </div>
    );
}

type AuthType = TargetFormValues['credentials']['auth_type'];

/**
 * The select value the form shows. The BESZEL mode is a UI concern: one backend
 * auth type (BESZEL) carries either a login pair or an API token, never both.
 */
type CredentialChoice =
    | 'NONE'
    | 'SNMP_V2C'
    | 'SNMP_V3'
    | 'HTTP_BASIC'
    | 'HTTP_BEARER'
    | 'DB_AUTH'
    | 'SMTP_LOGIN'
    | 'IMAP_LOGIN'
    | 'S3_KEY'
    | 'BESZEL_LOGIN'
    | 'BESZEL_TOKEN';

const authTypeByChoice: Record<CredentialChoice, AuthType> = {
    NONE: 'NONE',
    SNMP_V2C: 'SNMP_V2C',
    SNMP_V3: 'SNMP_V3',
    HTTP_BASIC: 'HTTP_BASIC',
    HTTP_BEARER: 'HTTP_BEARER',
    DB_AUTH: 'DB_AUTH',
    SMTP_LOGIN: 'MAIL_AUTH',
    IMAP_LOGIN: 'MAIL_AUTH',
    S3_KEY: 'S3_KEY',
    BESZEL_LOGIN: 'BESZEL',
    BESZEL_TOKEN: 'BESZEL',
};

const credentialOptionsByType: Record<TargetType, Array<{ value: CredentialChoice; label: string }>> = {
    NETWORK_DEVICE: [
        { value: 'NONE', label: 'No credentials' },
        { value: 'SNMP_V2C', label: 'SNMP v2c' },
        { value: 'SNMP_V3', label: 'SNMP v3' },
    ],
    INTERNET: [],
    WEB_APP: [
        { value: 'NONE', label: 'No credentials' },
        { value: 'HTTP_BASIC', label: 'Basic auth' },
        { value: 'HTTP_BEARER', label: 'Bearer token' },
    ],
    DOMAIN: [],
    DATABASE: [
        { value: 'NONE', label: 'No credentials' },
        { value: 'DB_AUTH', label: 'Database login' },
    ],
    MAIL_SERVER: [
        { value: 'NONE', label: 'No credentials' },
        { value: 'SMTP_LOGIN', label: 'SMTP login' },
        { value: 'IMAP_LOGIN', label: 'IMAP login' },
    ],
    S3_STORAGE: [
        { value: 'NONE', label: 'No credentials' },
        { value: 'S3_KEY', label: 'Access key' },
    ],
    BESZEL_SERVER: [
        { value: 'NONE', label: 'No credentials' },
        { value: 'BESZEL_LOGIN', label: 'Email and password' },
        { value: 'BESZEL_TOKEN', label: 'API token' },
    ],
};

const credentialFieldsByChoice: Record<CredentialChoice, Array<{ key: string; label: string; type?: string }>> = {
    NONE: [],
    SNMP_V2C: [{ key: 'community', label: 'Community string' }],
    SNMP_V3: [
        { key: 'username', label: 'Username (USM)' },
        { key: 'auth_protocol', label: 'Auth protocol (MD5 or SHA)' },
        { key: 'auth_password', label: 'Auth passphrase', type: 'password' },
        { key: 'priv_protocol', label: 'Priv protocol (DES or AES)' },
        { key: 'priv_password', label: 'Priv passphrase', type: 'password' },
    ],
    HTTP_BASIC: [
        { key: 'username', label: 'Username' },
        { key: 'password', label: 'Password', type: 'password' },
    ],
    HTTP_BEARER: [{ key: 'bearer_token', label: 'Bearer token', type: 'password' }],
    DB_AUTH: [
        { key: 'username', label: 'Database username' },
        { key: 'password', label: 'Database password', type: 'password' },
    ],
    SMTP_LOGIN: [
        { key: 'smtp_username', label: 'SMTP username' },
        { key: 'smtp_password', label: 'SMTP password', type: 'password' },
    ],
    IMAP_LOGIN: [
        { key: 'imap_username', label: 'IMAP username' },
        { key: 'imap_password', label: 'IMAP password', type: 'password' },
    ],
    S3_KEY: [
        { key: 'access_key', label: 'Access key' },
        { key: 'secret_key', label: 'Secret key', type: 'password' },
    ],
    BESZEL_LOGIN: [
        { key: 'email', label: 'Email', type: 'email' },
        { key: 'password', label: 'Password', type: 'password' },
    ],
    BESZEL_TOKEN: [{ key: 'api_token', label: 'API token', type: 'password' }],
};

const credentialHelperByType: Partial<Record<TargetType, string>> = {
    DATABASE: 'Use a read-only monitoring account, not root. For replication lag monitoring the account needs REPLICATION CLIENT.',
    S3_STORAGE: 'Use an IAM user with read-only access (HeadBucket, ListObjects), not a root key.',
    BESZEL_SERVER: 'Use a Beszel regular user added to the system, or an API token.',
};

/**
 * Per-target-type credential inputs. Hidden for DOMAIN, which has no credential
 * story. Switching the login method always resets the payload so a passphrase
 * from the previous method can never ride along into the request (R-26).
 */
function CredentialsSection({
    targetType,
    register,
    setValue,
    watch,
}: {
    targetType: TargetType;
    register: UseFormRegister<TargetFormValues>;
    setValue: UseFormSetValue<TargetFormValues>;
    watch: UseFormWatch<TargetFormValues>;
}) {
    const [choice, setChoice] = useState<CredentialChoice>('NONE');
    const options = credentialOptionsByType[targetType];

    // A target-type switch remounts this panel (TransitionPanel keys on the
    // type). Clear credentials left over from the previous type on mount.
    useEffect(() => {
        setChoice('NONE');
        setValue('credentials', { auth_type: 'NONE', payload: {} });
    }, [targetType, setValue]);

    // A form reset (after a successful save) writes auth_type back to NONE;
    // follow it so the select cannot show a method the form no longer holds.
    const authType = watch('credentials.auth_type');
    useEffect(() => {
        if (!authType || authType === 'NONE') {
            setChoice('NONE');
        }
    }, [authType]);

    if (options.length === 0) {
        return null;
    }

    const onChoiceChange = (value: string) => {
        const next = value as CredentialChoice;
        setChoice(next);
        setValue('credentials', { auth_type: authTypeByChoice[next], payload: {} });
    };

    const helper = credentialHelperByType[targetType];
    const fields = credentialFieldsByChoice[choice];

    return (
        <div className="space-y-3 rounded-sm border border-noc-border bg-white/[0.02] p-4">
            <p className="field-label">Credentials</p>
            <div>
                <label htmlFor="cred-auth-type" className="field-label mb-1.5 block">
                    Login method
                </label>
                <select id="cred-auth-type" className="input" value={choice} onChange={(e) => onChoiceChange(e.target.value)}>
                    {options.map((option) => (
                        <option key={option.value} value={option.value}>
                            {option.label}
                        </option>
                    ))}
                </select>
            </div>
            {fields.map((field) => (
                <div key={field.key}>
                    <label htmlFor={`cred-${field.key}`} className="field-label mb-1.5 block">
                        {field.label}
                    </label>
                    <input
                        id={`cred-${field.key}`}
                        type={field.type ?? 'text'}
                        {...register(`credentials.payload.${field.key}` as `credentials.payload.${string}`)}
                        className="input"
                    />
                </div>
            ))}
            {helper && <p className="text-[11px] text-noc-muted">{helper}</p>}
        </div>
    );
}

export { configSchemaByType };
