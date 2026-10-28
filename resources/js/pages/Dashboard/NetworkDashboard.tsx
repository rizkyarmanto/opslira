import { Head, Link } from '@inertiajs/react';
import ReactECharts from 'echarts-for-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import StatusBadge from '@/components/shared/StatusBadge';
import StatusCard from '@/components/beautiful-ui/StatusCard';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import SlidingNumber from '@/components/motion-primitives/SlidingNumber';
import { relativeTime } from '@/lib/time';
import { usePolling } from '@/lib/usePolling';
import type { MonitoringTarget } from '@/types/monitoring';

interface TrafficDevice {
    name: string;
    rx: Array<number | null>;
    tx: Array<number | null>;
}

interface Props {
    targets: MonitoringTarget[];
    internetTargets: MonitoringTarget[];
    trafficSeries: {
        times: string[];
        devices: TrafficDevice[];
        total: number;
        truncated: boolean;
    };
}

/**
 * Network view. The question this screen answers: which device carried traffic in
 * the last day, and which one went quiet? The chart is titled with that question,
 * and a five-minute bucket no probe filled is a gap rather than a line drawn at
 * zero (C-3, R-17).
 */
export default function NetworkDashboard({ targets, internetTargets, trafficSeries }: Props) {
    const { lastUpdated, refreshing } = usePolling(['targets', 'trafficSeries']);

    // The traffic chart reads SNMP from devices only, so Internet hosts are split
    // out before the chart and rendered as their own section below.
    const devices = targets.filter((t) => t.target_type !== 'INTERNET');
    const internet = internetTargets;

    const { times, devices: trafficDevices, total: trafficTotal, truncated: trafficTruncated } = trafficSeries;
    const hasTraffic = trafficDevices.length > 0;

    const trafficOption = {
        backgroundColor: 'transparent',
        grid: { left: 52, right: 20, top: 44, bottom: 34 },
        tooltip: {
            trigger: 'axis',
            backgroundColor: '#131316',
            borderColor: '#27272a',
            textStyle: { color: '#f4f4f5', fontSize: 12 },
        },
        legend: {
            textStyle: { color: '#a1a1aa', fontSize: 11 },
            top: 0,
            type: 'scroll',
            icon: 'roundRect',
        },
        xAxis: {
            type: 'category',
            data: times,
            boundaryGap: false,
            axisLine: { lineStyle: { color: '#3f3f46' } },
            axisLabel: { color: '#a1a1aa', fontSize: 10, hideOverlap: true },
        },
        yAxis: {
            type: 'value',
            name: 'Mbps',
            nameTextStyle: { color: '#a1a1aa', fontSize: 10 },
            axisLabel: { color: '#a1a1aa', fontSize: 10 },
            splitLine: { lineStyle: { color: '#27272a' } },
        },
        // One line per device and direction: In reads the receive counter, Out the
        // transmit counter, both averaged into the same five-minute bucket.
        series: trafficDevices.flatMap((device) => [
            {
                name: `${device.name} in`,
                type: 'line',
                smooth: true,
                showSymbol: false,
                connectNulls: false,
                data: device.rx,
                itemStyle: { color: '#34d399' },
                lineStyle: { color: '#34d399', width: 1.5 },
            },
            {
                name: `${device.name} out`,
                type: 'line',
                smooth: true,
                showSymbol: false,
                connectNulls: false,
                data: device.tx,
                itemStyle: { color: '#38bdf8' },
                lineStyle: { color: '#38bdf8', width: 1.5 },
            },
        ]),
    };

    return (
        <AppLayout title="Network">
            <Head title="Network" />

            <div className="mb-5 flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <p className="text-xs text-noc-muted">
                    {refreshing ? (
                        <ReloadIndicator label="Refreshing status" />
                    ) : lastUpdated ? (
                        `Updated ${lastUpdated.toLocaleTimeString('en-US')}`
                    ) : (
                        'Loading latest status...'
                    )}
                </p>
                <p className="text-xs text-noc-muted">
                    {devices.length} devices and {internet.length} Internet hosts monitored.
                </p>
            </div>

            <Card className="mb-6 px-4 py-4">
                <h2 className="text-sm font-semibold text-noc-text">
                    Traffic per device, last 24 hours
                </h2>
                {hasTraffic ? (
                    <>
                        <ReactECharts option={trafficOption} style={{ height: 280 }} />
                        {trafficTruncated && (
                            <p className="mt-1 text-[10px] text-noc-muted">
                                Showing {trafficDevices.length} of {trafficTotal} devices that reported traffic.
                            </p>
                        )}
                    </>
                ) : (
                    <p className="py-8 text-center text-sm text-noc-muted">
                        No SNMP readings in the last 24 hours. Numbers appear after a network probe
                        returns interface data.
                    </p>
                )}
            </Card>

            {/* Network devices: SNMP-capable hardware */}
            <section className="mb-8" aria-labelledby="devices-heading">
                <div className="mb-3 flex items-center gap-2">
                    <h2 id="devices-heading" className="text-sm font-semibold text-noc-text">
                        Network devices
                    </h2>
                    <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                        {devices.length}
                    </span>
                </div>

                {devices.length === 0 ? (
                    <Card className="px-4 py-8 text-center">
                        <p className="text-sm text-noc-muted">No network devices registered yet.</p>
                        <Link href="/targets" className="btn-primary mt-3">
                            Open the Targets page
                        </Link>
                    </Card>
                ) : (
                    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
                        {devices.map((target) => {
                            const metrics = (target.state?.summary_metrics ?? {}) as Record<string, number>;
                            const counts = deviceCounts(target.config_payload?.sub_type, metrics);

                            return (
                                <Link key={target.id} href={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                                <StatusCard
                                    status={target.current_status}
                                    heartbeat={target.heartbeat}
                                    header={
                                        <>
                                            <div className="flex items-center gap-2">
                                                <p className="truncate text-sm font-medium text-noc-text">
                                                    {target.name}
                                                </p>
                                            </div>
                                            <p className="tabular mt-1 truncate text-xs text-noc-muted">
                                                {target.host_or_endpoint}
                                            </p>
                                        </>
                                    }
                                    footer={
                                        <div className="flex items-center justify-between gap-2">
                                            <ProtocolBadge protocolType={target.protocol_type} />
                                            <span className="text-[10px] text-noc-muted">
                                                {relativeTime(target.last_checked_at) ?? 'Not yet checked'}
                                            </span>
                                        </div>
                                    }
                                >
                                    <div className="mb-3 flex items-center justify-between gap-2">
                                        <StatusBadge status={target.current_status} />
                                        {target.config_payload?.sub_type != null ? (
                                            <span className="tabular text-[10px] text-noc-muted">
                                                {String(target.config_payload.sub_type)}
                                            </span>
                                        ) : null}
                                    </div>

                                    <div className="grid grid-cols-2 gap-3">
                                        <Metric
                                            label="Latency"
                                            value={metrics.last_latency_ms ?? target.state?.last_latency_ms ?? null}
                                            unit=" ms"
                                        />
                                        <Metric
                                            label="Packet loss"
                                            value={metrics.packet_loss_pct ?? null}
                                            unit="%"
                                        />
                                    </div>

                                    <div className="mt-3 space-y-2">
                                        <ProgressBar label="CPU" value={metrics.cpu_usage_pct ?? null} />
                                        <ProgressBar label="Memory" value={metrics.mem_usage_pct ?? null} />
                                    </div>

                                    {counts !== null && (
                                        <p className="tabular mt-2 text-[10px] text-noc-muted">{counts}</p>
                                    )}
                                </StatusCard>
                                </Link>
                            );
                        })}
                    </div>
                )}
            </section>

            {/* Internet hosts: reachability, latency and packet loss */}
            <section aria-labelledby="network-internet-heading">
                <div className="mb-3 flex items-center gap-2">
                    <h2 id="network-internet-heading" className="text-sm font-semibold text-noc-text">
                        Internet
                    </h2>
                    <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">
                        {internet.length}
                    </span>
                </div>

                {internet.length === 0 ? (
                    <Card className="px-4 py-8 text-center">
                        <p className="text-sm text-noc-muted">No Internet hosts registered yet.</p>
                        <Link href="/targets" className="btn-primary mt-3">
                            Open the Targets page
                        </Link>
                    </Card>
                ) : (
                    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
                        {internet.map((target) => {
                            const metrics = (target.state?.summary_metrics ?? {}) as Record<string, number>;

                            return (
                                <Link key={target.id} href={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                                <StatusCard
                                    status={target.current_status}
                                    heartbeat={target.heartbeat}
                                    header={
                                        <>
                                            <div className="flex items-center gap-2">
                                                <p className="truncate text-sm font-medium text-noc-text">
                                                    {target.name}
                                                </p>
                                            </div>
                                            <p className="tabular mt-1 truncate text-xs text-noc-muted">
                                                {target.host_or_endpoint}
                                            </p>
                                        </>
                                    }
                                    footer={
                                        <div className="flex items-center justify-between gap-2">
                                            <ProtocolBadge protocolType={target.protocol_type} />
                                            <span className="text-[10px] text-noc-muted">
                                                {relativeTime(target.last_checked_at) ?? 'Not yet checked'}
                                            </span>
                                        </div>
                                    }
                                >
                                    <div className="mb-3 flex items-center justify-between gap-2">
                                        <StatusBadge status={target.current_status} />
                                    </div>

                                    <div className="grid grid-cols-2 gap-3">
                                        <Metric
                                            label="Latency"
                                            value={target.state?.last_latency_ms ?? null}
                                            unit=" ms"
                                        />
                                        <Metric
                                            label="Packet loss"
                                            value={metrics.packet_loss_pct ?? null}
                                            unit="%"
                                        />
                                    </div>
                                </StatusCard>
                                </Link>
                            );
                        })}
                    </div>
                )}
            </section>
        </AppLayout>
    );
}

function Metric({ label, value, unit }: { label: string; value: number | null; unit: string }) {
    return (
        <div className="rounded-sm border border-noc-border bg-white/[0.02] px-3 py-2">
            <p className="field-label">{label}</p>
            <p className="mt-0.5 text-lg font-semibold text-noc-text">
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
                    <span>no data</span>
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

/**
 * One muted line naming the counters this device's probe actually produces: a
 * router reports PPP tunnels, a switch reports PoE. A device that has none of
 * them shows no line at all, never a zero (R-17).
 */
function deviceCounts(subType: unknown, metrics: Record<string, unknown>): string | null {
    const count = (key: string): number | null => {
        const value = metrics[key];

        return typeof value === 'number' && Number.isFinite(value) ? value : null;
    };

    const parts: string[] = [];

    if (subType === 'ROUTER') {
        const up = count('ppp_tunnels_up');
        const total = count('ppp_tunnels_total');

        if (up !== null && total !== null) {
            parts.push(`PPP ${up}/${total}`);
        }
    }

    if (subType === 'SWITCH') {
        const powered = count('poe_ports_powered');
        const total = count('poe_ports_total');

        // A device can report PoE power without the per-port table (HP 2530); a
        // placeholder 0/0 would read as "no port is powered", which is a claim
        // the probe never measured. Only a real per-port count earns the line.
        if (powered !== null && total !== null && total > 0) {
            parts.push(`PoE ${powered}/${total}`);
        }
    }

    const portsUp = count('ports_up');
    const portsTotal = count('ports_total');

    if (portsUp !== null && portsTotal !== null) {
        parts.push(`Port ${portsUp}/${portsTotal}`);
    }

    return parts.length === 0 ? null : parts.join(' · ');
}
