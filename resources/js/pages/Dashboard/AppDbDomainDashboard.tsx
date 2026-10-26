import { Head, Link } from '@inertiajs/react';
import { Database, Globe, Lock } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import StatusBadge from '@/components/shared/StatusBadge';
import AnimatedTabs, { type TabItem } from '@/components/ui-layouts/AnimatedTabs';
import ExpiryCountdownCard from '@/components/beautiful-ui/ExpiryCountdownCard';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
import SlidingNumber from '@/components/motion-primitives/SlidingNumber';
import NoData from '@/components/shared/NoData';
import { relativeTime } from '@/lib/time';
import { usePolling } from '@/lib/usePolling';
import type { MonitoringTarget } from '@/types/monitoring';

interface Props {
    targets: MonitoringTarget[];
    environment: string;
    counts: Record<string, number>;
}

/**
 * Applications, domains, and databases. The screen is split by what the operator is
 * checking: is the app answering, is anything about to expire, is the database
 * reachable. Tabs filter by environment through real navigation (R-26).
 */
export default function AppDbDomainDashboard({ targets, environment, counts }: Props) {
    const { lastUpdated } = usePolling(['targets', 'environment']);

    const tabs: TabItem[] = [
        { id: 'ALL', label: 'All', count: counts.ALL },
        { id: 'PRODUCTION', label: 'Production', count: counts.PRODUCTION },
        { id: 'STAGING', label: 'Staging', count: counts.STAGING },
        { id: 'DEVELOPMENT', label: 'Development', count: counts.DEVELOPMENT },
    ];

    const apps = targets.filter((t) => t.target_type === 'WEB_APP');
    const domains = targets.filter((t) => t.target_type === 'DOMAIN');
    const databases = targets.filter((t) => t.target_type === 'DATABASE');

    return (
        <AppLayout title="Apps, Domains & Databases">
            <Head title="Apps, Domains & Databases" />

            <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
                <AnimatedTabs
                    tabs={tabs}
                    active={environment}
                    buildHref={(id) => (id === 'ALL' ? '/dashboard/apps-domains-db' : `/dashboard/apps-domains-db?environment=${id}`)}
                />
                <p className="text-xs text-noc-muted">
                    {lastUpdated
                        ? `Updated ${lastUpdated.toLocaleTimeString('en-US')}`
                        : 'Loading latest status...'}
                </p>
            </div>

            <Section title="Web apps" icon={<Globe size={15} aria-hidden />} count={apps.length} emptyHint="No web apps match this filter.">
                {apps.map((target) => {
                    const metrics = (target.state?.summary_metrics ?? {}) as Record<string, number>;
                    const httpStatus = metrics.http_status;

                    return (
                        <Link key={target.id} href={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                        <Card className="p-5">
                            <div className="mb-2 flex items-start justify-between gap-3">
                                <div className="min-w-0">
                                    <span className="truncate text-sm font-medium text-noc-text">
                                        {target.name}
                                    </span>
                                    <p className="tabular truncate text-xs text-noc-muted">{target.host_or_endpoint}</p>
                                </div>
                                <StatusBadge status={target.current_status} />
                            </div>

                            <div className="mt-3 flex flex-wrap items-center gap-2">
                                {httpStatus ? (
                                    <span
                                        className={`tabular rounded-xs border px-2 py-0.5 text-[11px] font-semibold ${
                                            httpStatus < 400 ? 'status-up' : 'status-down'
                                        }`}
                                    >
                                        HTTP {httpStatus}
                                    </span>
                                ) : (
                                    <span className="rounded-xs border border-noc-border px-2 py-0.5 text-[11px] text-noc-muted">
                                        HTTP not yet checked
                                    </span>
                                )}
                                <ProtocolBadge protocolType={target.protocol_type} />
                            </div>

                            <div className="mt-4 flex items-baseline justify-between">
                                <span className="field-label">Response time</span>
                                <span className="text-lg font-semibold text-noc-text">
                                    <SlidingNumber value={target.state?.last_latency_ms ?? null} decimals={1} suffix=" ms" />
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

            <Section title="Domains & TLS certificates" icon={<Lock size={15} aria-hidden />} count={domains.length} emptyHint="No domains match this filter.">
                {domains.map((target) => {
                    const metrics = (target.state?.summary_metrics ?? {}) as Record<string, string | number>;

                    return (
                        <Link key={target.id} href={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                        <ExpiryCountdownCard
                            name={target.name}
                            status={target.current_status}
                            domainName={target.host_or_endpoint}
                            domainDaysLeft={(metrics.domain_days_left as number) ?? null}
                            domainExpiryDate={(metrics.domain_expiry_date as string) ?? null}
                            sslDaysLeft={(metrics.ssl_days_left as number) ?? null}
                            sslExpiryDate={(metrics.ssl_expiry_date as string) ?? null}
                            sslIssuer={(metrics.ssl_issuer as string) ?? null}
                        />
                        </Link>
                    );
                })}
            </Section>

            <Section title="Databases" icon={<Database size={15} aria-hidden />} count={databases.length} emptyHint="No databases match this filter.">
                {databases.map((target) => {
                    const metrics = (target.state?.summary_metrics ?? {}) as Record<string, number>;

                    return (
                        <Link key={target.id} href={`/targets/${target.id}`} className="block rounded-md transition-colors hover:border-noc-muted focus-visible:outline-2 focus-visible:outline-noc-accent" aria-label={`View details for ${target.name}`}>
                        <Card className="p-5">
                            <div className="mb-2 flex items-start justify-between gap-3">
                                <div className="min-w-0">
                                    <span className="truncate text-sm font-medium text-noc-text">
                                        {target.name}
                                    </span>
                                    <p className="tabular truncate text-xs text-noc-muted">
                                        {target.host_or_endpoint}
                                        {target.port ? `:${target.port}` : ''}
                                    </p>
                                </div>
                                <StatusBadge status={target.current_status} />
                            </div>

                            <div className="mt-4 grid grid-cols-2 gap-3">
                                <div>
                                    <p className="field-label">Query time</p>
                                    <p className="tabular mt-0.5 text-lg font-semibold text-noc-text">
                                        <SlidingNumber value={metrics.query_exec_ms ?? null} decimals={1} suffix=" ms" />
                                    </p>
                                </div>
                                <div>
                                    <p className="field-label">Active connections</p>
                                    <p className="tabular mt-0.5 text-lg font-semibold text-noc-text">
                                        {metrics.active_connections ?? <NoData />}
                                    </p>
                                </div>
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
