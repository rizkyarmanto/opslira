import { useState } from 'react';
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
import TargetFormDrawer from '@/pages/Targets/TargetFormDrawer';
import { formatDateTime, relativeTime } from '@/lib/time';
import { api } from '@/lib/api';
import type { Incident, MonitoringTarget, TargetState } from '@/types/monitoring';
interface Props {
    target: MonitoringTarget & { state: TargetState; has_credentials: boolean };
    logs: Array<{
        id: number;
        status: string;
        response_time_ms: number | null;
        checked_at: string;
        error_message: string | null;
    }>;
    incidents: Incident[];
}

/**
 * Target detail: the evidence behind a status verdict. The chart answers one
 * question, how response time has moved over the recent checks, so its title states
 * that question (C-3). Logs are newest first with named columns (R-20).
 */
export default function TargetDetail({ target, logs, incidents }: Props) {
    const [editing, setEditing] = useState(false);
    const [busy, setBusy] = useState(false);
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
                    <h2 className="mb-3 text-sm font-semibold text-noc-text">Incident history</h2>
                    {incidents.length === 0 ? (
                        <p className="py-4 text-center text-xs text-noc-muted">
                            No incidents recorded for this target.
                        </p>
                    ) : (
                        <ul className="space-y-2">
                            {incidents.map((i) => (
                                <li key={i.id} className="rounded-sm border border-noc-border px-3 py-2">
                                    <div className="flex items-center justify-between gap-2">
                                        <span className="text-xs font-semibold text-noc-text">
                                            {i.severity === 'CRITICAL' ? 'Critical' : 'Warning'}
                                        </span>
                                        <span className="text-[10px] text-noc-muted">{i.status}</span>
                                    </div>
                                    <p className="mt-1 text-xs text-noc-muted">{i.root_cause_message}</p>
                                    <p className="tabular mt-1 text-[10px] text-noc-muted">
                                        {formatDateTime(i.started_at) ?? ''}
                                        {i.resolved_at ? ` until ${formatDateTime(i.resolved_at)}` : ', not yet resolved'}
                                    </p>
                                </li>
                            ))}
                        </ul>
                    )}
                </Card>

                <Card className="p-5">
                    <h2 className="mb-3 text-sm font-semibold text-noc-text">Check log</h2>
                    {logs.length === 0 ? (
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
                                    {logs.slice(0, 50).map((log) => (
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
                    )}
                </Card>
            </div>
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

            // A router and a switch answer different SNMP objects: the router adds
            // health sensors and PPP tunnels, the switch adds PoE ports (R-17, C-3).
            if (subType === 'ROUTER' || subType === 'SWITCH') {
                return (
                    <div className="grid grid-cols-1 gap-x-6 gap-y-4 sm:grid-cols-2">
                        {identity}
                        {reachability}
                        {subType === 'ROUTER' && (
                            <div className="space-y-2 sm:col-span-2">
                                <ProgressBar label="CPU" value={num(m.cpu_usage_pct)} />
                                <ProgressBar label="Memory" value={num(m.mem_usage_pct)} />
                            </div>
                        )}
                        <DevicePorts metrics={m} />
                        {subType === 'ROUTER' ? <RouterExtras metrics={m} /> : <SwitchPoe metrics={m} />}
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

/** Interfaces listed per device before the list is cut short with a note. */
const DISPLAYED_PORTS = 24;

type InterfaceRow = {
    if_index: number;
    name: string;
    oper: string | null;
    speed_mbps: number | null;
    rx_mbps: number | null;
    tx_mbps: number | null;
};

type PoePortRow = {
    pse: string;
    detection: string | null;
};

/** summary_metrics.ports arrives as JSON; keep only rows the chip can render. */
function interfaceRows(value: unknown): InterfaceRow[] {
    if (!Array.isArray(value)) {
        return [];
    }

    return value.flatMap((row) => {
        if (typeof row !== 'object' || row === null) {
            return [];
        }

        const raw = row as Record<string, unknown>;
        const name = str(raw.name);
        const ifIndex = num(raw.if_index);

        return name === null || ifIndex === null
            ? []
            : [
                  {
                      if_index: ifIndex,
                      name,
                      oper: str(raw.oper),
                      speed_mbps: num(raw.speed_mbps),
                      rx_mbps: num(raw.rx_mbps),
                      tx_mbps: num(raw.tx_mbps),
                  },
              ];
    });
}

function poePortRows(value: unknown): PoePortRow[] {
    if (!Array.isArray(value)) {
        return [];
    }

    return value.flatMap((row) => {
        if (typeof row !== 'object' || row === null) {
            return [];
        }

        const raw = row as Record<string, unknown>;
        const pse = str(raw.pse) ?? (num(raw.pse) !== null ? String(num(raw.pse)) : null);

        return pse === null ? [] : [{ pse, detection: str(raw.detection) }];
    });
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

/** Interface table: counts first, then the ports themselves. */
function DevicePorts({ metrics }: { metrics: Record<string, unknown> }) {
    const ports = interfaceRows(metrics.ports);
    const shown = ports.slice(0, DISPLAYED_PORTS);
    const total = num(metrics.ports_total);
    const up = num(metrics.ports_up);
    const down = num(metrics.ports_down);

    return (
        <div className="space-y-2 sm:col-span-2">
            <p className="field-label">Ports</p>
            {total === null ? (
                <NoData />
            ) : (
                <p className="tabular text-sm font-semibold text-noc-text">
                    {up ?? 0} up, {down ?? 0} down of {total}
                </p>
            )}
            {shown.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
                    {shown.map((port) => (
                        <InterfaceChip key={port.if_index} port={port} />
                    ))}
                </div>
            )}
            {(metrics.ports_truncated === true || ports.length > shown.length) && (
                <p className="text-[10px] text-noc-muted">
                    Showing the first {shown.length} of {total ?? ports.length} interfaces.
                </p>
            )}
        </div>
    );
}

/** Router-only facts: hardware sensors and PPP tunnels. */
function RouterExtras({ metrics }: { metrics: Record<string, unknown> }) {
    const temperature = num(metrics.sensor_temp_c);
    const voltage = num(metrics.sensor_voltage_v);
    const tunnelsUp = num(metrics.ppp_tunnels_up);
    const tunnelsTotal = num(metrics.ppp_tunnels_total);

    return (
        <>
            {metrics.sensor_supported === true ? (
                <>
                    <Fact label="Temperature">
                        {temperature === null ? <NoData /> : `${temperature.toFixed(1)} C`}
                    </Fact>
                    <Fact label="Voltage">
                        {voltage === null ? <NoData /> : `${voltage.toFixed(1)} V`}
                    </Fact>
                </>
            ) : (
                <Fact label="Hardware sensors">
                    <NoData />
                </Fact>
            )}
            <Fact label="PPP tunnels">
                {tunnelsTotal === null ? <NoData /> : `${tunnelsUp ?? 0} up of ${tunnelsTotal}`}
            </Fact>
        </>
    );
}

/** Switch-only facts: PoE ports, or an honest empty state when the device has none. */
function SwitchPoe({ metrics }: { metrics: Record<string, unknown> }) {
    const allPorts = poePortRows(metrics.poe_ports);
    const ports = allPorts.slice(0, DISPLAYED_PORTS);
    const powered = num(metrics.poe_ports_powered);
    const total = num(metrics.poe_ports_total);
    const budget = num(metrics.poe_total_watts);

    return (
        <div className="space-y-2 sm:col-span-2">
            <p className="field-label">PoE</p>
            {metrics.poe_supported !== true ? (
                <p className="text-xs text-noc-muted">This device does not report PoE.</p>
            ) : (
                <>
                    {total !== null && total > 0 ? (
                        <p className="tabular text-sm font-semibold text-noc-text">
                            {powered ?? 0} powered of {total}
                        </p>
                    ) : (
                        <p className="text-xs text-noc-muted">
                            This device reports PoE power but not state per port.
                        </p>
                    )}
                    {budget !== null && (
                        <p className="tabular text-[11px] text-noc-muted">PoE budget {budget} W</p>
                    )}
                    {ports.length > 0 && (
                        <div className="flex flex-wrap gap-1.5">
                            {ports.map((port) => (
                                <PoeChip key={port.pse} port={port} />
                            ))}
                        </div>
                    )}
                    {(metrics.poe_truncated === true || allPorts.length > ports.length) && (
                        <p className="text-[10px] text-noc-muted">
                            Showing the first {ports.length} of {total ?? 0} PoE ports.
                        </p>
                    )}
                </>
            )}
        </div>
    );
}

function InterfaceChip({ port }: { port: InterfaceRow }) {
    const checked = port.oper !== null;
    const up = port.oper === 'up';
    const classes = !checked
        ? 'border-noc-border text-noc-muted'
        : up
          ? 'border-status-up/30 bg-status-up/10 text-status-up'
          : 'border-status-down/40 bg-status-down/10 text-status-down';

    const details: string[] = [];

    if (port.speed_mbps !== null && port.speed_mbps > 0) {
        details.push(`${port.speed_mbps} Mbps`);
    }

    if (port.rx_mbps !== null && port.tx_mbps !== null) {
        details.push(`rx ${port.rx_mbps} tx ${port.tx_mbps} Mbps`);
    }

    return (
        <span className={`tabular inline-flex items-center gap-1.5 rounded-xs border px-2 py-0.5 text-[11px] ${classes}`}>
            <span className="font-semibold">{port.name}</span>
            <span>{!checked ? 'not yet checked' : up ? 'up' : 'down'}</span>
            {details.length > 0 && <span className="text-noc-muted">{details.join(' / ')}</span>}
        </span>
    );
}

/** Detection state carries the color, the word carries the meaning. */
const POE_STATE_CLASSES: Record<string, string> = {
    delivering: 'border-status-up/30 bg-status-up/10 text-status-up',
    searching: 'border-status-degraded/40 bg-status-degraded/10 text-status-degraded',
    fault: 'border-status-down/40 bg-status-down/10 text-status-down',
    other_fault: 'border-status-down/40 bg-status-down/10 text-status-down',
};

function PoeChip({ port }: { port: PoePortRow }) {
    const state = port.detection ?? 'unknown';
    const classes = POE_STATE_CLASSES[state] ?? 'border-noc-border text-noc-muted';

    return (
        <span className={`tabular inline-flex items-center rounded-xs border px-2 py-0.5 text-[11px] ${classes}`}>
            {port.pse}: {state.replace('_', ' ')}
        </span>
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
