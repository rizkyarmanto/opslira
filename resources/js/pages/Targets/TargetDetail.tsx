import { Head, Link } from '@inertiajs/react';
import ReactECharts from 'echarts-for-react';
import { ArrowLeft } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import StatusBadge from '@/components/shared/StatusBadge';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
import NoData from '@/components/shared/NoData';
import { formatDateTime, relativeTime } from '@/lib/time';
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
    const ordered = [...logs].reverse();
    const times = ordered.map((l) => new Date(l.checked_at).toLocaleTimeString('en-US'));
    const latencies = ordered.map((l) => l.response_time_ms);

    const hasLatency = latencies.some((v) => v !== null);

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
        </AppLayout>
    );
}
