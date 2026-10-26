import { Link } from 'react-router-dom';
import { Boxes, Mail, ShieldAlert, ShieldCheck } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import StatusBadge from '@/components/shared/StatusBadge';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
import SlidingNumber from '@/components/motion-primitives/SlidingNumber';
import NoData from '@/components/shared/NoData';
import { Head } from '@/lib/head';
import { relativeTime } from '@/lib/time';
import type { MonitoringTarget } from '@/types/monitoring';

interface Props {
    targets: MonitoringTarget[];
    /** Time of the last successful load, for the "Updated" label. */
    lastUpdated: Date | null;
}

/**
 * Mail, object storage, and Beszel-monitored servers.
 *
 * Blacklist state is reported only from what the probe actually checked. When the
 * probe has not run, the panel says so rather than claiming a clean result (R-17,
 * R-38). Container counts come from Beszel and are shown only when present.
 */
export default function MailS3BeszelDashboard({ targets, lastUpdated }: Props) {
    const mail = targets.filter((t) => t.target_type === 'MAIL_SERVER');
    const s3 = targets.filter((t) => t.target_type === 'S3_STORAGE');
    const beszel = targets.filter((t) => t.target_type === 'BESZEL_SERVER');

    return (
        <AppLayout title="Mail, S3 & Beszel">
            <Head title="Mail, S3 & Beszel" />

            <div className="mb-5 flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <p className="text-xs text-noc-muted">
                    {lastUpdated
                        ? `Updated ${lastUpdated.toLocaleTimeString('en-US')}`
                        : 'Loading latest status...'}
                </p>
                <p className="text-xs text-noc-muted">{targets.length} targets in this group.</p>
            </div>

            <Section
                title="Mail server"
                icon={<Mail size={15} aria-hidden />}
                count={mail.length}
                emptyHint="No mail servers registered."
            >
                {mail.map((target) => {
                    const m = (target.state?.summary_metrics ?? {}) as Record<string, unknown>;
                    const cfg = (target.config_payload ?? {}) as Record<string, any>;
                    const mode = cfg.mail_check_mode === 'imap' ? 'imap' : 'smtp';
                    const legacyPorts = (cfg.check_ports ?? {}) as Record<string, number>;
                    const mailPort = Number(target.port ?? legacyPorts[mode] ?? (mode === 'smtp' ? cfg.smtp_port : cfg.imap_port) ?? (mode === 'imap' ? 993 : 587));
                    const blacklisted = m.is_blacklisted === true;
                    const listedOn = (m.listed_on as string[]) ?? [];
                    const checked = m.dnsbl_checked === true || m.is_blacklisted !== undefined;

                    return (
                        <Link key={target.id} to={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                        <Card className="p-5">
                            <div className="mb-3 flex items-start justify-between gap-3">
                                <div className="min-w-0">
                                    <span className="tabular truncate text-sm font-medium text-noc-text">
                                        {target.host_or_endpoint}
                                    </span>
                                    <p className="truncate text-xs text-noc-muted">{target.name}</p>
                                </div>
                                <StatusBadge status={target.current_status} />
                            </div>

                            <div className="flex flex-wrap gap-2">
                                <PortChip
                                    label={`${mode === 'imap' ? 'IMAP' : 'SMTP'} ${mailPort}`}
                                    up={m[`${mode}_${mailPort}_up`] === true}
                                    checked={m[`${mode}_${mailPort}_up`] !== undefined}
                                />
                            </div>

                            <div
                                className={`mt-3 flex items-start gap-2 rounded-sm border px-3 py-2 text-xs ${
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

                            <p className="mt-2 text-[10px] text-noc-muted">
                                {relativeTime(target.last_checked_at) ?? 'Not yet checked'}
                            </p>
                        </Card>
                        </Link>
                    );
                })}
            </Section>

            <Section
                title="Object storage (S3)"
                icon={<Boxes size={15} aria-hidden />}
                count={s3.length}
                emptyHint="No buckets registered."
            >
                {s3.map((target) => {
                    const m = (target.state?.summary_metrics ?? {}) as Record<string, number>;
                    const bucket = (target.config_payload?.bucket_name as string) ?? target.name;

                    return (
                        <Link key={target.id} to={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                        <Card className="p-5">
                            <div className="mb-2 flex items-start justify-between gap-3">
                                <div className="min-w-0">
                                    <span className="truncate text-sm font-medium text-noc-text">
                                        {bucket}
                                    </span>
                                    <p className="tabular truncate text-xs text-noc-muted">{target.host_or_endpoint}</p>
                                </div>
                                <StatusBadge status={target.current_status} />
                            </div>

                            <div className="mt-4 flex items-baseline justify-between">
                                <span className="field-label">HeadBucket time</span>
                                <span className="text-lg font-semibold text-noc-text">
                                    <SlidingNumber value={m.head_bucket_ms ?? null} decimals={1} suffix=" ms" />
                                </span>
                            </div>
                            <div className="mt-2 flex items-center justify-between">
                                <ProtocolBadge protocolType={target.protocol_type} />
                                <span className="text-[10px] text-noc-muted">
                                    {relativeTime(target.last_checked_at) ?? 'Not yet checked'}
                                </span>
                            </div>
                        </Card>
                        </Link>
                    );
                })}
            </Section>

            <Section
                title="Beszel servers"
                icon={<Boxes size={15} aria-hidden />}
                count={beszel.length}
                emptyHint="No Beszel servers registered."
            >
                {beszel.map((target) => {
                    const m = (target.state?.summary_metrics ?? {}) as Record<string, number>;
                    const hasContainers = m.containers_running !== undefined || m.containers_unhealthy !== undefined;

                    return (
                        <Link key={target.id} to={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                        <Card className="p-5">
                            <div className="mb-4 flex items-start justify-between gap-3">
                                <div className="min-w-0">
                                    <span className="truncate text-sm font-medium text-noc-text">
                                        {target.name}
                                    </span>
                                    <p className="tabular truncate text-xs text-noc-muted">{target.host_or_endpoint}</p>
                                </div>
                                <StatusBadge status={target.current_status} />
                            </div>

                            <div className="flex items-center justify-around">
                                <RadialGauge label="CPU" value={m.cpu_pct ?? null} />
                                <RadialGauge label="RAM" value={m.mem_pct ?? null} />
                                <RadialGauge label="Disk" value={m.disk_pct ?? null} />
                            </div>

                            <div className="mt-4 flex items-center justify-between border-t border-noc-border pt-3">
                                <span className="field-label">Docker containers</span>
                                <span className="tabular text-xs text-noc-text">
                                    {hasContainers
                                        ? `${m.containers_running ?? 0} running${
                                              m.containers_unhealthy ? `, ${m.containers_unhealthy} unhealthy` : ''
                                          }`
                                        : <NoData />}
                                </span>
                            </div>
                            <p className="mt-2 text-[10px] text-noc-muted">
                                {relativeTime(target.last_checked_at) ?? 'Not yet checked'}
                            </p>
                        </Card>
                        </Link>
                    );
                })}
            </Section>
        </AppLayout>
    );
}

function Section({
    title,
    icon,
    count,
    emptyHint,
    children,
}: {
    title: string;
    icon: React.ReactNode;
    count: number;
    emptyHint: string;
    children: React.ReactNode;
}) {
    return (
        <section className="mb-8">
            <div className="mb-3 flex items-center gap-2">
                <span className="text-noc-muted">{icon}</span>
                <h2 className="text-sm font-semibold text-noc-text">{title}</h2>
                <span className="tabular rounded-xs bg-white/5 px-2 py-0.5 text-[11px] text-noc-muted">{count}</span>
            </div>
            {count === 0 ? (
                <p className="rounded-md border border-dashed border-noc-border px-4 py-6 text-center text-xs text-noc-muted">
                    {emptyHint}
                </p>
            ) : (
                <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">{children}</div>
            )}
        </section>
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
                {hasValue ? <SlidingNumber value={value} decimals={0} suffix="%" /> : <NoData />}
            </span>
            <span className="field-label">{label}</span>
        </div>
    );
}
