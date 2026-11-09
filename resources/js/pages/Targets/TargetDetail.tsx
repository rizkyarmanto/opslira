import { Fragment, useCallback, useEffect, useRef, useState } from 'react';
import type { ReactNode } from 'react';
import { Head, Link, router } from '@inertiajs/react';
import ReactECharts from 'echarts-for-react';
import { ArrowLeft, ShieldAlert, ShieldCheck, Pencil } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Button from '@/components/shared/Button';
import Card from '@/components/shared/Card';
import StatusBadge from '@/components/shared/StatusBadge';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
import ExpiryCountdownCard from '@/components/beautiful-ui/ExpiryCountdownCard';
import SlidingNumber from '@/components/motion-primitives/SlidingNumber';
import NoData from '@/components/shared/NoData';
import ConfirmModal from '@/components/shared/ConfirmModal';
import SlideOverDrawer from '@/components/ui-layouts/SlideOverDrawer';
import TargetFormDrawer from '@/pages/Targets/TargetFormDrawer';
import { formatDateTime, relativeTime } from '@/lib/time';
import { api, apiErrorMessage } from '@/lib/api';
import type { Incident, MonitoringTarget, TargetState } from '@/types/monitoring';
interface Props {
    target: MonitoringTarget & { state: TargetState; has_credentials: boolean };
    logs: Array<{
        id: number;
        status: string;
        response_time_ms: number | null;
        checked_at: string;
    }>;
    incidents: Incident[];
    /** Full incident count for this target; the card renders at most five. */
    incident_total: number;
}

/** One row from the Degraded history tab: a check that came back DEGRADED. */
interface DegradedLog {
    id: number;
    status: string;
    response_time_ms: number | null;
    checked_at: string;
    error_message: string | null;
}

type ActivityRow = {
    username: string | null;
    database: string | null;
    state: string | null;
    state_duration: string | null;
    query_age: string | null;
    query_syntax: string | null;
};

/** Check log views. Checks is the probe feed; Degraded history filters it. */
const logTabs: Array<{ key: 'checks' | 'degraded'; label: string }> = [
    { key: 'checks', label: 'Checks' },
    { key: 'degraded', label: 'Degraded history' },
];

/**
 * Target detail: the evidence behind a status verdict. The chart answers one
 * question, how response time has moved over the recent checks, so its title states
 * that question (C-3). Logs are newest first with named columns (R-20).
 */
export default function TargetDetail({ target, logs, incidents, incident_total }: Props) {
    const [editing, setEditing] = useState(false);
    const [busy, setBusy] = useState(false);
    const [resolveTarget, setResolveTarget] = useState<Incident | null>(null);
    const [resolving, setResolving] = useState(false);
    const [resolveError, setResolveError] = useState<string | null>(null);
    const [showAllIncidents, setShowAllIncidents] = useState(false);
    const [allIncidents, setAllIncidents] = useState<Incident[] | null>(null);
    const [loadingIncidents, setLoadingIncidents] = useState(false);
    const [incidentError, setIncidentError] = useState<string | null>(null);
    const [logTab, setLogTab] = useState<'checks' | 'degraded'>('checks');
    const [degradedLogs, setDegradedLogs] = useState<DegradedLog[] | null>(null);
    const [degradedLoading, setDegradedLoading] = useState(false);
    const [degradedError, setDegradedError] = useState<string | null>(null);
    const ordered = [...logs].reverse();
    const times = ordered.map((l) => new Date(l.checked_at).toLocaleTimeString('en-US'));
    const latencies = ordered.map((l) => l.response_time_ms);

    const hasLatency = latencies.some((v) => v !== null);
    const enableTarget = async () => {
        setBusy(true);
        try {
            await api.post(`/targets/${target.id}/enable`);
            router.reload();
        } finally {
            setBusy(false);
        }
    };

    const openResolve = (incident: Incident) => {
        setResolveError(null);
        setResolveTarget(incident);
    };

    // The drawer fetches the full history on first open and keeps it while the
    // panel stays mounted, so re-opening costs nothing until a resolve reloads it.
    const loadAllIncidents = async () => {
        setLoadingIncidents(true);
        setIncidentError(null);
        try {
            const res = await api.get(`/targets/${target.id}/incidents`);
            setAllIncidents(res.data.data as Incident[]);
        } catch (error) {
            setIncidentError(apiErrorMessage(error, 'Could not load the incident history.'));
        } finally {
            setLoadingIncidents(false);
        }
    };

    const openAllIncidents = () => {
        setShowAllIncidents(true);
        if (allIncidents === null) void loadAllIncidents();
    };

    const loadDegradedLogs = async () => {
        setDegradedLoading(true);
        setDegradedError(null);
        try {
            const res = await api.get(`/targets/${target.id}/degraded-logs`);
            setDegradedLogs(res.data.data as DegradedLog[]);
        } catch (error) {
            setDegradedError(apiErrorMessage(error, 'Could not load degraded checks.'));
        } finally {
            setDegradedLoading(false);
        }
    };

    const selectLogTab = (tab: 'checks' | 'degraded') => {
        setLogTab(tab);
        if (tab === 'degraded' && degradedLogs === null) void loadDegradedLogs();
    };

    const confirmResolve = async () => {
        if (!resolveTarget) return;
        setResolving(true);
        try {
            await api.post(`/incidents/${resolveTarget.id}/resolve`);
            setResolveTarget(null);
            router.reload();
            // The page reload refreshes the card and its count, not the drawer's
            // own fetched list, so that list is refetched while it is on screen.
            if (showAllIncidents) void loadAllIncidents();
        } catch (error) {
            setResolveError(apiErrorMessage(error, 'Could not resolve this incident.'));
        } finally {
            setResolving(false);
        }
    };

    const option = {
        backgroundColor: 'transparent',
        grid: { left: 48, right: 20, top: 16, bottom: 32 },
        tooltip: {
            trigger: 'axis',
            backgroundColor: '#131316',
            borderColor: '#27272a',
            textStyle: { color: '#f4f4f5', fontSize: 12 },
        },
        xAxis: {
            type: 'category',
            data: times,
            axisLine: { lineStyle: { color: '#3f3f46' } },
            axisLabel: { color: '#a1a1aa', fontSize: 10 },
        },
        yAxis: {
            type: 'value',
            name: 'ms',
            nameTextStyle: { color: '#a1a1aa', fontSize: 10 },
            axisLabel: { color: '#a1a1aa', fontSize: 10 },
            splitLine: { lineStyle: { color: '#27272a' } },
        },
        series: [
            {
                type: 'line',
                smooth: true,
                symbolSize: 4,
                data: latencies,
                connectNulls: false,
                itemStyle: { color: '#34d399' },
                lineStyle: { color: '#34d399' },
            },
        ],
    };

    return (
        <AppLayout>
            <Head title={target.name} />

            <Link href="/targets" className="mb-4 inline-flex items-center gap-1.5 text-sm text-noc-muted hover:text-noc-text">
                <ArrowLeft size={15} aria-hidden />
                Back to target list
            </Link>

            <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h1 className="font-display text-display-sm">{target.name}</h1>
                    <p className="tabular mt-1 text-sm text-noc-muted">
                        {target.host_or_endpoint}
                        {target.port ? `:${target.port}` : ''}
                    </p>
                </div>
                <div className="flex items-center gap-2">
                    {!target.is_active && (
                        <Button variant="ghost" loading={busy} onClick={enableTarget} className="!px-3 !py-1.5 !text-xs">
                            Enable
                        </Button>
                    )}
                    <Button variant="ghost" onClick={() => setEditing(true)} className="!px-3 !py-1.5 !text-xs">
                        <Pencil size={14} aria-hidden /> Edit
                    </Button>
                    <ProtocolBadge protocolType={target.protocol_type} />
                    <StatusBadge status={target.current_status} />
                </div>
            </div>
            <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <Card className="p-4">
                    <p className="field-label">Environment</p>
                    <p className="mt-1 text-sm text-noc-text">{target.environment}</p>
                </Card>
                <Card className="p-4">
                    <p className="field-label">Last check</p>
                    <p className="mt-1 text-sm text-noc-text">
                        {relativeTime(target.last_checked_at) ?? 'Not checked yet'}
                    </p>
                </Card>
                <Card className="p-4">
                    <p className="field-label">Check interval</p>
                    <p className="tabular mt-1 text-sm text-noc-text">{target.check_interval_seconds} seconds</p>
                </Card>
                <Card className="p-4">
                    <p className="field-label">Credentials</p>
                    <p className="mt-1 text-sm text-noc-text">
                        {target.has_credentials ? 'Stored, encrypted' : 'None'}
                    </p>
                </Card>
            </div>

            <Card className="mb-6 p-5">
                <h2 className="text-sm font-semibold text-noc-text">Target facts</h2>
                <div className="mt-3">
                    <TargetFacts target={target} />
                </div>
            </Card>

            <Card className="mb-6 p-5">
                <h2 className="text-sm font-semibold text-noc-text">
                    Response time, last {ordered.length} checks
                </h2>
                {hasLatency ? (
                    <ReactECharts option={option} style={{ height: 240 }} />
                ) : (
                    <p className="py-10 text-center text-sm text-noc-muted">
                        No response time recorded yet. Run a probe for this target to populate the chart.
                    </p>
                )}
            </Card>

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                <Card className="p-5">
                    <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                        <h2 className="text-sm font-semibold text-noc-text">Incident history</h2>
                        {incident_total > 0 && (
                            <span className="tabular text-[11px] text-noc-muted">
                                {incident_total > incidents.length
                                    ? `Showing ${incidents.length} of ${incident_total}`
                                    : `${incident_total} total`}
                            </span>
                        )}
                    </div>
                    {incidents.length === 0 ? (
                        <p className="py-4 text-center text-xs text-noc-muted">
                            No incidents recorded for this target.
                        </p>
                    ) : (
                        <ul className="space-y-2">
                            {incidents.map((i) => (
                                <IncidentRow key={i.id} incident={i} onResolve={openResolve} />
                            ))}
                        </ul>
                    )}
                    {incident_total > 5 && (
                        <Button variant="ghost" className="mt-3 !px-3 !py-1.5 !text-xs" onClick={openAllIncidents}>
                            View all incidents
                        </Button>
                    )}
                </Card>

                <Card className="p-5">
                    <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                        <h2 className="text-sm font-semibold text-noc-text">Check log</h2>
                        <div role="group" aria-label="Check log view" className="flex items-center gap-1">
                            {logTabs.map((tab) => (
                                <button
                                    key={tab.key}
                                    type="button"
                                    aria-pressed={logTab === tab.key}
                                    onClick={() => selectLogTab(tab.key)}
                                    className={`rounded-xs px-2.5 py-1 text-xs transition-colors ${
                                        logTab === tab.key
                                            ? 'bg-white/5 font-medium text-noc-text'
                                            : 'text-noc-muted hover:text-noc-text'
                                    }`}
                                >
                                    {tab.label}
                                </button>
                            ))}
                        </div>
                    </div>
                    {logTab === 'checks' ? (
                        logs.length === 0 ? (
                            <p className="py-4 text-center text-xs text-noc-muted">
                                No logs yet. No probe has run for this target.
                            </p>
                        ) : (
                            <div className="max-h-80 overflow-y-auto">
                                <table className="w-full text-left text-xs">
                                    <thead className="sticky top-0 bg-noc-surface">
                                        <tr>
                                            <th scope="col" className="field-label py-2">Time</th>
                                            <th scope="col" className="field-label py-2">Status</th>
                                            <th scope="col" className="field-label py-2 text-right">Response</th>
                                        </tr>
                                    </thead>
                                    <tbody className="divide-y divide-noc-border">
                                        {logs.map((log) => (
                                            <tr key={log.id}>
                                                <td className="tabular py-1.5 text-noc-muted">
                                                    {new Date(log.checked_at).toLocaleString('en-US')}
                                                </td>
                                                <td className="py-1.5">
                                                    <StatusBadge status={log.status as never} />
                                                </td>
                                                <td className="tabular py-1.5 text-right text-noc-muted">
                                                    {log.response_time_ms !== null ? `${log.response_time_ms} ms` : <NoData />}
                                                </td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        )
                    ) : degradedLoading ? (
                        <p className="py-4 text-center text-xs text-noc-muted">Loading degraded checks...</p>
                    ) : degradedError ? (
                        <p role="alert" className="py-4 text-center text-xs text-status-down">
                            {degradedError}
                        </p>
                    ) : degradedLogs && degradedLogs.length > 0 ? (
                        <div className="max-h-80 overflow-y-auto">
                            <table className="w-full text-left text-xs">
                                <thead className="sticky top-0 bg-noc-surface">
                                    <tr>
                                        <th scope="col" className="field-label py-2">Time</th>
                                        <th scope="col" className="field-label py-2">Status</th>
                                        <th scope="col" className="field-label py-2 text-right">Response</th>
                                    </tr>
                                </thead>
                                <tbody className="divide-y divide-noc-border">
                                    {degradedLogs.map((log) => (
                                        <Fragment key={log.id}>
                                            <tr>
                                                <td className="tabular py-1.5 text-noc-muted">
                                                    {new Date(log.checked_at).toLocaleString('en-US')}
                                                </td>
                                                <td className="py-1.5">
                                                    <StatusBadge status={log.status as never} />
                                                </td>
                                                <td className="tabular py-1.5 text-right text-noc-muted">
                                                    {log.response_time_ms !== null ? `${log.response_time_ms} ms` : <NoData />}
                                                </td>
                                            </tr>
                                            {log.error_message && (
                                                <tr>
                                                    <td colSpan={3} className="pb-2 text-[11px] text-noc-muted">
                                                        {log.error_message}
                                                    </td>
                                                </tr>
                                            )}
                                        </Fragment>
                                    ))}
                                </tbody>
                            </table>
                        </div>
                    ) : (
                        <p className="py-4 text-center text-xs text-noc-muted">
                            No degraded checks recorded for this target.
                        </p>
                    )}
                </Card>
            </div>
            <SlideOverDrawer
                open={showAllIncidents}
                onClose={() => setShowAllIncidents(false)}
                title="All incidents"
                description={`History for ${target.name}`}
                footer={<p className="text-xs text-noc-muted">Showing newest 200</p>}
            >
                {loadingIncidents ? (
                    <p className="py-6 text-center text-sm text-noc-muted">Loading incidents...</p>
                ) : incidentError ? (
                    <p role="alert" className="py-6 text-center text-sm text-status-down">
                        {incidentError}
                    </p>
                ) : allIncidents && allIncidents.length > 0 ? (
                    <ul className="space-y-2">
                        {allIncidents.map((i) => (
                            <IncidentRow key={i.id} incident={i} onResolve={openResolve} />
                        ))}
                    </ul>
                ) : (
                    <p className="py-6 text-center text-sm text-noc-muted">
                        No incidents recorded for this target.
                    </p>
                )}
            </SlideOverDrawer>

            <ConfirmModal
                tone="info"
                open={resolveTarget !== null}
                title="Resolve incident"
                message={`Mark the incident for "${target.name}" as resolved? The target keeps its probe status.`}
                confirmLabel="Resolve"
                pending={resolving}
                error={resolveError ?? undefined}
                onConfirm={confirmResolve}
                onClose={() => setResolveTarget(null)}
            />

            <TargetFormDrawer
                open={editing}
                onClose={() => setEditing(false)}
                onSaved={() => router.reload()}
                editing={target}
                hasCredentials={target.has_credentials}
            />
        </AppLayout>
    );
}

function num(value: unknown): number | null {
    return typeof value === 'number' && Number.isFinite(value) ? value : null;
}

function str(value: unknown): string | null {
    return typeof value === 'string' && value !== '' ? value : null;
}

/** One incident line, shared by the history card and its all-incidents drawer. */
function IncidentRow({ incident, onResolve }: { incident: Incident; onResolve: (incident: Incident) => void }) {
    return (
        <li className="rounded-sm border border-noc-border px-3 py-2">
            <div className="flex items-center justify-between gap-2">
                <span className="text-xs font-semibold text-noc-text">
                    {incident.severity === 'CRITICAL' ? 'Critical' : 'Warning'}
                </span>
                <div className="flex items-center gap-2">
                    <span className="text-[10px] text-noc-muted">{incident.status}</span>
                    {incident.status !== 'RESOLVED' && (
                        <Button
                            variant="ghost"
                            className="!px-2 !py-1 !text-[11px]"
                            onClick={() => onResolve(incident)}
                        >
                            Resolve
                        </Button>
                    )}
                </div>
            </div>
            <p className="mt-1 text-xs text-noc-muted">{incident.root_cause_message}</p>
            <p className="tabular mt-1 text-[10px] text-noc-muted">
                {formatDateTime(incident.started_at) ?? ''}
                {incident.resolved_at ? ` until ${formatDateTime(incident.resolved_at)}` : ', not yet resolved'}
            </p>
        </li>
    );
}

/**
 * Per-type evidence: the summary_metrics keys each probe adapter actually writes.
 * A key the probe has not produced renders n/a, never a zero (R-17).
 */
function TargetFacts({ target }: { target: Props['target'] }) {
    const m = (target.state?.summary_metrics ?? {}) as Record<string, unknown>;
    const cfg = (target.config_payload ?? {}) as Record<string, unknown>;

    switch (target.target_type) {
        case 'DOMAIN':
            return (
                <ExpiryCountdownCard
                    name={target.name}
                    status={target.current_status}
                    domainName={target.host_or_endpoint}
                    domainDaysLeft={num(m.domain_days_left)}
                    domainExpiryDate={str(m.domain_expiry_date)}
                    sslDaysLeft={num(m.ssl_days_left)}
                    sslExpiryDate={str(m.ssl_expiry_date)}
                    sslIssuer={str(m.ssl_issuer)}
                />
            );

        case 'WEB_APP': {
            const httpStatus = num(m.http_status);

            return (
                <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                    <Fact label="HTTP status">
                        {httpStatus !== null ? (
                            <span
                                className={`tabular inline-flex rounded-xs border px-2 py-0.5 text-[11px] font-semibold ${
                                    httpStatus < 400 ? 'status-up' : 'status-down'
                                }`}
                            >
                                HTTP {httpStatus}
                            </span>
                        ) : (
                            <span className="inline-flex rounded-xs border border-noc-border px-2 py-0.5 text-[11px] font-normal text-noc-muted">
                                HTTP not yet checked
                            </span>
                        )}
                    </Fact>
                    <Fact label="Response time">
                        <SlidingNumber value={target.state?.last_latency_ms ?? null} decimals={1} suffix=" ms" />
                    </Fact>
                </div>
            );
        }

        case 'NETWORK_DEVICE': {
            const subType = target.config_payload?.sub_type != null ? String(target.config_payload.sub_type) : null;
            const identity = <DeviceIdentity metrics={m} />;
            const reachability = (
                <>
                    <Metric
                        label="Latency"
                        value={num(m.last_latency_ms) ?? target.state?.last_latency_ms ?? null}
                        unit=" ms"
                    />
                    <Metric label="Packet loss" value={num(m.packet_loss_pct)} unit="%" />
                </>
            );

            // A router answers hardware sensor OIDs that a switch does not, so only
            // the router carries that block; both share identity and reachability.
            if (subType === 'ROUTER' || subType === 'SWITCH') {
                return (
                    <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                        {identity}
                        {reachability}
                        {subType === 'ROUTER' && (
                            <>
                                <div className="space-y-2 sm:col-span-2">
                                    <ProgressBar label="CPU" value={num(m.cpu_usage_pct)} />
                                    <ProgressBar label="Memory" value={num(m.mem_usage_pct)} />
                                </div>
                                <RouterSensors metrics={m} />
                            </>
                        )}
                    </div>
                );
            }

            return (
                <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                    {subType && (
                        <p className="text-[10px] text-noc-muted sm:col-span-2">Device type: {subType}</p>
                    )}
                    {reachability}
                    <div className="space-y-2 sm:col-span-2">
                        <ProgressBar label="CPU" value={num(m.cpu_usage_pct)} />
                        <ProgressBar label="Memory" value={num(m.mem_usage_pct)} />
                    </div>
                </div>
            );
        }

        case 'INTERNET':
            return (
                <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                    <Metric label="Latency" value={target.state?.last_latency_ms ?? null} unit=" ms" />
                    <Metric label="Packet loss" value={num(m.packet_loss_pct)} unit="%" />
                </div>
            );

        case 'DATABASE':
            return (
                <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                    <Fact label="Query time">
                        <SlidingNumber value={num(m.query_exec_ms)} decimals={1} suffix=" ms" />
                    </Fact>
                    <Fact label="Active connections">{num(m.active_connections) ?? <NoData />}</Fact>
                    {target.protocol_type === 'DB_NATIVE_PING' && (
                        <DatabaseActivity key={target.id} targetId={target.id} />
                    )}
                </div>
            );

        case 'MAIL_SERVER': {
            const mode = cfg.mail_check_mode === 'imap' ? 'imap' : 'smtp';
            const legacyPorts = (cfg.check_ports ?? {}) as Record<string, number>;
            const mailPort = Number(
                target.port ??
                    legacyPorts[mode] ??
                    (mode === 'smtp' ? cfg.smtp_port : cfg.imap_port) ??
                    (mode === 'imap' ? 993 : 587),
            );
            const blacklisted = m.is_blacklisted === true;
            const listedOn = Array.isArray(m.listed_on) ? (m.listed_on as string[]) : [];
            const checked = m.dnsbl_checked === true || m.is_blacklisted !== undefined;

            return (
                <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                    <Fact label="Port check">
                        <PortChip
                            label={`${mode === 'imap' ? 'IMAP' : 'SMTP'} ${mailPort}`}
                            up={m[`${mode}_${mailPort}_up`] === true}
                            checked={m[`${mode}_${mailPort}_up`] !== undefined}
                        />
                    </Fact>
                    <div
                        className={`flex items-start gap-2 rounded-sm border px-3 py-2 text-xs sm:col-span-2 ${
                            !checked
                                ? 'border-noc-border bg-white/[0.02] text-noc-muted'
                                : blacklisted
                                  ? 'border-status-down/40 bg-status-down/10 text-status-down'
                                  : 'border-status-up/30 bg-status-up/10 text-status-up'
                        }`}
                    >
                        {blacklisted ? (
                            <ShieldAlert size={14} className="mt-px shrink-0" aria-hidden />
                        ) : (
                            <ShieldCheck size={14} className="mt-px shrink-0" aria-hidden />
                        )}
                        <span>
                            {!checked
                                ? 'DNSBL check has not run yet.'
                                : blacklisted
                                  ? `Listed on blacklist: ${listedOn.join(', ') || 'provider did not name a list'}`
                                  : 'Not listed on any checked DNSBL provider.'}
                        </span>
                    </div>
                </div>
            );
        }

        case 'S3_STORAGE': {
            const bucket = str(cfg.bucket_name) ?? target.name;

            return (
                <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                    <Fact label="Bucket">{bucket}</Fact>
                    <Fact label="HeadBucket time">
                        <SlidingNumber value={num(m.head_bucket_ms)} decimals={1} suffix=" ms" />
                    </Fact>
                </div>
            );
        }

        case 'BESZEL_SERVER': {
            const hasContainers = m.containers_running !== undefined || m.containers_unhealthy !== undefined;
            const running = num(m.containers_running);
            const unhealthy = num(m.containers_unhealthy);

            return (
                <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                    <div className="flex items-center justify-around sm:col-span-2">
                        <RadialGauge label="CPU" value={num(m.cpu_pct)} />
                        <RadialGauge label="RAM" value={num(m.mem_pct)} />
                        <RadialGauge label="Disk" value={num(m.disk_pct)} />
                    </div>
                    <Fact label="Docker containers">
                        {hasContainers ? (
                            `${running ?? 0} running${unhealthy ? `, ${unhealthy} unhealthy` : ''}`
                        ) : (
                            <NoData />
                        )}
                    </Fact>
                </div>
            );
        }

        default:
            return <NoData />;
    }
}

function DatabaseActivity({ targetId }: { targetId: Props['target']['id'] }) {
    const [rows, setRows] = useState<ActivityRow[]>([]);
    const [collectedAt, setCollectedAt] = useState<string | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const request = useRef<AbortController | null>(null);

    const refresh = useCallback(async () => {
        request.current?.abort();
        const controller = new AbortController();
        request.current = controller;
        setLoading(true);
        setError(null);
        try {
            const response = await api.get<{ data: ActivityRow[]; collected_at: string }>(
                `/targets/${targetId}/database-activity`, { signal: controller.signal },
            );
            if (controller.signal.aborted) return;
            setRows(response.data.data);
            setCollectedAt(response.data.collected_at);
        } catch (err) {
            if (controller.signal.aborted) return;
            setRows([]);
            setCollectedAt(null);
            setError(apiErrorMessage(err, 'Could not load database activity. Check the connection and monitoring permissions.'));
        } finally {
            if (!controller.signal.aborted) setLoading(false);
        }
    }, [targetId]);

    useEffect(() => {
        void refresh();
        return () => request.current?.abort();
    }, [refresh]);

    return (
        <section className="min-w-0 sm:col-span-2" aria-label="Database activity">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                <div>
                    <h3 className="text-sm font-semibold text-noc-text">Database activity</h3>
                    {collectedAt && <p className="text-xs text-noc-muted">Collected {formatDateTime(collectedAt)}</p>}
                </div>
                <Button variant="ghost" className="!px-3 !py-1.5 !text-xs" onClick={() => void refresh()} disabled={loading}>
                    Refresh
                </Button>
            </div>
            {loading ? (
                <p className="py-4 text-xs text-noc-muted">Loading database activity...</p>
            ) : error ? (
                <p role="alert" className="py-4 text-xs text-status-down">{error}</p>
            ) : rows.length === 0 ? (
                <p className="py-4 text-xs text-noc-muted">No database sessions found.</p>
            ) : (
                <div className="max-w-full overflow-x-auto">
                    <table className="w-full text-left text-xs">
                        <thead>
                            <tr>
                                {['Username', 'Database', 'State', 'State duration', 'Query age', 'Query syntax'].map((label) => (
                                    <th key={label} scope="col" className="field-label whitespace-nowrap px-2 py-2">{label}</th>
                                ))}
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-noc-border">
                            {rows.map((row, index) => (
                                <tr key={index}>
                                    {[row.username, row.database, row.state, row.state_duration, row.query_age].map((value, column) => (
                                        <td key={column} className="whitespace-nowrap px-2 py-2 text-noc-text">{value ?? <NoData />}</td>
                                    ))}
                                    <td className="px-2 py-2 text-noc-text">
                                        {row.query_syntax === null ? <NoData /> : (
                                            <pre className="max-h-32 max-w-xs overflow-auto whitespace-pre-wrap break-words">{row.query_syntax}</pre>
                                        )}
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            )}
        </section>
    );
}

function truncate(value: string, limit: number): string {
    return value.length <= limit ? value : `${value.slice(0, limit - 1)}...`;
}

/** sysUpTime seconds as a short reading, never a raw seven digit number. */
function formatUptime(seconds: number | null): string | null {
    if (seconds === null) {
        return null;
    }

    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);

    if (days > 0) {
        return `${days}d ${hours}h ${minutes}m`;
    }

    return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`;
}

/** SNMP identity block, shared by router and switch. */
function DeviceIdentity({ metrics }: { metrics: Record<string, unknown> }) {
    const name = str(metrics.device_sys_name);
    const description = str(metrics.device_sys_descr);
    const uptimeText = formatUptime(num(metrics.device_uptime_seconds));

    return (
        <div className="space-y-1 sm:col-span-2">
            <p className="field-label">Identity</p>
            <p className="text-sm font-semibold text-noc-text">{name ?? <NoData />}</p>
            {description !== null && <p className="text-xs text-noc-muted">{truncate(description, 120)}</p>}
            <p className="tabular text-[11px] text-noc-muted">
                Uptime: {uptimeText ?? <NoData />}
            </p>
        </div>
    );
}

/** Router-only facts: hardware sensors. */
function RouterSensors({ metrics }: { metrics: Record<string, unknown> }) {
    const temperature = num(metrics.sensor_temp_c);
    const voltage = num(metrics.sensor_voltage_v);

    if (metrics.sensor_supported !== true) {
        return (
            <Fact label="Hardware sensors">
                <NoData />
            </Fact>
        );
    }

    return (
        <>
            <Fact label="Temperature">
                {temperature === null ? <NoData /> : `${temperature.toFixed(1)} C`}
            </Fact>
            <Fact label="Voltage">
                {voltage === null ? <NoData /> : `${voltage.toFixed(1)} V`}
            </Fact>
        </>
    );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
    return (
        <div>
            <p className="field-label">{label}</p>
            <p className="tabular mt-0.5 text-sm font-semibold text-noc-text">{children}</p>
        </div>
    );
}

function Metric({ label, value, unit }: { label: string; value: number | null; unit: string }) {
    return (
        <div className="rounded-sm border border-noc-border bg-white/[0.02] px-3 py-2">
            <p className="field-label">{label}</p>
            <p className="tabular mt-0.5 text-lg font-semibold text-noc-text">
                <SlidingNumber value={value} decimals={1} suffix={unit} />
            </p>
        </div>
    );
}

function ProgressBar({ label, value }: { label: string; value: number | null }) {
    if (value === null) {
        return (
            <div>
                <div className="mb-1 flex justify-between text-[10px] text-noc-muted">
                    <span>{label}</span>
                    <NoData />
                </div>
                <div className="h-1.5 w-full rounded-full bg-white/5" />
            </div>
        );
    }

    const pct = Math.max(0, Math.min(100, value));
    const color = pct >= 85 ? 'bg-status-down' : pct >= 70 ? 'bg-status-degraded' : 'bg-status-up';

    return (
        <div>
            <div className="mb-1 flex justify-between text-[10px] text-noc-muted">
                <span>{label}</span>
                <span className="tabular">{pct.toFixed(1)}%</span>
            </div>
            <div className="h-1.5 w-full overflow-hidden rounded-full bg-white/5">
                <div className={`h-full rounded-full ${color}`} style={{ width: `${pct}%` }} />
            </div>
        </div>
    );
}

function PortChip({ label, up, checked }: { label: string; up: boolean; checked: boolean }) {
    const classes = !checked
        ? 'border-noc-border text-noc-muted'
        : up
          ? 'border-status-up/30 bg-status-up/10 text-status-up'
          : 'border-status-down/40 bg-status-down/10 text-status-down';

    return (
        <span className={`tabular inline-flex items-center rounded-xs border px-2 py-0.5 text-[11px] ${classes}`}>
            {label}: {!checked ? 'not yet checked' : up ? 'open' : 'closed'}
        </span>
    );
}

function RadialGauge({ label, value }: { label: string; value: number | null }) {
    const hasValue = value !== null;
    const pct = Math.max(0, Math.min(100, value ?? 0));
    const color = pct >= 85 ? '#fb7185' : pct >= 70 ? '#fbbf24' : '#34d399';
    const circumference = 2 * Math.PI * 26;
    const offset = circumference - (pct / 100) * circumference;

    return (
        <div className="flex flex-col items-center">
            <svg width="64" height="64" viewBox="0 0 64 64" className="-rotate-90" aria-hidden>
                <circle cx="32" cy="32" r="26" fill="none" stroke="#27272a" strokeWidth="6" />
                {hasValue && (
                    <circle
                        cx="32"
                        cy="32"
                        r="26"
                        fill="none"
                        stroke={color}
                        strokeWidth="6"
                        strokeLinecap="round"
                        strokeDasharray={circumference}
                        strokeDashoffset={offset}
                    />
                )}
            </svg>
            <span className="tabular mt-1 text-sm font-semibold text-noc-text">
                <SlidingNumber value={value} decimals={0} suffix="%" />
            </span>
            <span className="field-label">{label}</span>
        </div>
    );
}
