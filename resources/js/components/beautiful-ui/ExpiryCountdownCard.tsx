import { AlertTriangle } from 'lucide-react';
import StatusBadge from '@/components/shared/StatusBadge';
import NoData from '@/components/shared/NoData';
import type { TargetStatus } from '@/types/monitoring';

/**
 * Domain and TLS expiry, with a bar that reads as a countdown to a real date.
 *
 * The bar fills as the deadline approaches, so a nearly expired certificate is the
 * fullest bar, which is the thing the operator needs to see. When the probe has not
 * run, the value is an em dash and the bar is empty, never a green zero (R-17).
 *
 * Thresholds: more than 30 days is healthy, 15 to 30 warns, 14 or fewer is critical.
 */
export default function ExpiryCountdownCard({
    name,
    status,
    domainName,
    domainDaysLeft,
    domainExpiryDate,
    sslDaysLeft,
    sslExpiryDate,
    sslIssuer,
}: {
    name: string;
    status: TargetStatus;
    domainName: string;
    domainDaysLeft: number | null;
    domainExpiryDate: string | null;
    sslDaysLeft: number | null;
    sslExpiryDate: string | null;
    sslIssuer?: string | null;
}) {
    return (
        <div className="rounded-md border border-noc-border bg-noc-surface p-5">
            <div className="mb-4 flex items-start justify-between gap-3">
                <div className="min-w-0">
                    <p className="truncate text-sm font-medium text-noc-text">{name}</p>
                    <p className="tabular truncate text-xs text-noc-muted">{domainName}</p>
                </div>
                <StatusBadge status={status} />
            </div>

            <CountdownBar label="Domain" days={domainDaysLeft} date={domainExpiryDate} />
            <div className="h-4" />
            <CountdownBar label="TLS Certificate" days={sslDaysLeft} date={sslExpiryDate} issuer={sslIssuer} />
        </div>
    );
}

function CountdownBar({
    label,
    days,
    date,
    issuer,
}: {
    label: string;
    days: number | null;
    date: string | null;
    issuer?: string | null;
}) {
    // Bar fills as expiry approaches: 0 days left = full, 365 days left = empty.
    const pct = days === null ? 0 : Math.max(0, Math.min(100, ((365 - Math.min(days, 365)) / 365) * 100));

    const tone =
        days === null
            ? { bar: 'bg-white/10', text: 'text-noc-muted', note: null }
            : days > 30
              ? { bar: 'bg-status-up', text: 'text-noc-text', note: null }
              : days > 14
                ? { bar: 'bg-status-degraded', text: 'text-status-degraded', note: 'Expiring soon' }
                : { bar: 'bg-status-down', text: 'text-status-down', note: 'Critical' };

    return (
        <div>
            <div className="mb-1.5 flex items-baseline justify-between gap-2">
                <span className="field-label">{label}</span>
                <span className={`tabular text-sm font-semibold ${tone.text}`}>
                    {days === null ? <NoData /> : `${days} days`}
                </span>
            </div>

            <div className="h-1.5 w-full overflow-hidden rounded-full bg-white/5">
                <div className={`h-full rounded-full ${tone.bar}`} style={{ width: `${pct}%` }} />
            </div>

            <div className="mt-1.5 flex items-baseline justify-between gap-2">
                <span className="tabular text-[10px] text-noc-muted">
                    {date ? `Expires ${date}` : 'Date not yet known'}
                </span>
                {tone.note && (
                    <span className={`inline-flex items-center gap-1 text-[10px] font-semibold ${tone.text}`}>
                        <AlertTriangle size={10} aria-hidden />
                        {tone.note}
                    </span>
                )}
            </div>

            {issuer && <p className="mt-0.5 text-[10px] text-noc-muted">Issuer: {issuer}</p>}
        </div>
    );
}
