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

interface Props {
    targets: MonitoringTarget[];
    internetTargets: MonitoringTarget[];
    trafficSummary: Record<string, { rx_mbps: number; tx_mbps: number }>;
}

/**
 * Network view. The question this screen answers: are the office links carrying
 * traffic, and which device is not? The chart is titled with that question, and a
 * device with no SNMP reading says so instead of drawing a flat line at zero
 * (C-3, R-17).
 */
export default function NetworkDashboard({ targets, internetTargets, trafficSummary }: Props) {
    const { lastUpdated, refreshing } = usePolling(['targets', 'trafficSummary']);

    // The traffic chart reads SNMP from devices only, so Internet hosts are split
    // out before the chart and rendered as their own section below.
    const devices = targets.filter((t) => t.target_type !== 'INTERNET');
    const internet = internetTargets;

    const deviceNames = new Set(devices.map((d) => d.name));
    const series = Object.entries(trafficSummary).filter(
        ([name, v]) => deviceNames.has(name) && (v.rx_mbps > 0 || v.tx_mbps > 0),
    );
    const names = series.map(([name]) => name);
    const hasTraffic = series.length > 0;

    const trafficOption = {
        backgroundColor: 'transparent',
        grid: { left: 48, right: 20, top: 36, bottom: 36 },
        tooltip: {
            trigger: 'axis',
            backgroundColor: '#131316',
            borderColor: '#27272a',
            textStyle: { color: '#f4f4f5', fontSize: 12 },
        },
        legend: {
            data: ['In', 'Out'],
            textStyle: { color: '#a1a1aa', fontSize: 11 },
            top: 0,
            icon: 'roundRect',
        },
        xAxis: {
            type: 'category',
            data: names,
            axisLine: { lineStyle: { color: '#3f3f46' } },
            axisLabel: { color: '#a1a1aa', fontSize: 10, interval: 0, rotate: names.length > 4 ? 20 : 0 },
        },
        yAxis: {
            type: 'value',
            name: 'Mbps',
            nameTextStyle: { color: '#a1a1aa', fontSize: 10 },
            axisLabel: { color: '#a1a1aa', fontSize: 10 },
            splitLine: { lineStyle: { color: '#27272a' } },
        },
        series: [
            {
                name: 'In',
                type: 'line',
                smooth: true,
                symbolSize: 5,
                data: series.map(([, v]) => v.rx_mbps),
                itemStyle: { color: '#34d399' },
                lineStyle: { color: '#34d399' },
            },
            {
                name: 'Out',
                type: 'line',
                smooth: true,
                symbolSize: 5,
                data: series.map(([, v]) => v.tx_mbps),
                itemStyle: { color: '#38bdf8' },
                lineStyle: { color: '#38bdf8' },
            },
        ],
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
                    Traffic per device, latest SNMP reading
                </h2>
                {hasTraffic ? (
                    <ReactECharts option={trafficOption} style={{ height: 260 }} />
                ) : (
                    <p className="py-8 text-center text-sm text-noc-muted">
                        No SNMP readings yet. Numbers appear after the network probe runs and the device
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

                            return (
                                <StatusCard
                                    key={target.id}
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
                                </StatusCard>
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
                                <StatusCard
                                    key={target.id}
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
