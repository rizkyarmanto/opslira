import type { TargetStatus } from '@/types/monitoring';

const protocolLabels: Record<string, string> = {
    ICMP_SNMP: 'ICMP + SNMP',
    HTTP_HEALTH: 'cURL /health',
    WHOIS_SSL: 'WHOIS + SSL',
    DB_NATIVE_PING: 'Native DB',
    SMTP_DNSBL: 'SMTP + DNSBL',
    S3_HEAD: 'S3 SDK',
    BESZEL_API: 'Beszel API',
};

/**
 * Protocol chip. Neutral by design: the protocol is a fact about the target, not a
 * state, so it carries no status dot (R-31). The check method is named because the
 * label is what the operator is looking for, not a color.
 */
export default function ProtocolBadge({ protocolType }: { protocolType: string }) {
    const label = protocolLabels[protocolType] ?? protocolType;

    return (
        <span className="inline-flex shrink-0 items-center rounded-xs border border-noc-border bg-white/5 px-2 py-0.5 font-mono text-[11px] text-noc-muted">
            {label}
        </span>
    );
}

const deliveryStyles: Record<string, string> = {
    SENT: 'status-up',
    PENDING: 'status-pending',
    RETRYING: 'status-degraded',
    FAILED: 'status-down',
};

const deliveryLabels: Record<string, string> = {
    SENT: 'SENT',
    PENDING: 'PENDING',
    RETRYING: 'RETRYING',
    FAILED: 'FAILED',
};

export function DeliveryBadge({ status }: { status: string }) {
    return (
        <span
            className={`inline-flex shrink-0 items-center rounded-full border px-2 py-0.5 text-[11px] font-semibold ${
                deliveryStyles[status] ?? deliveryStyles.PENDING
            }`}
        >
            {deliveryLabels[status] ?? status}
        </span>
    );
}

export function statusAccent(status: TargetStatus): string {
    return {
        UP: 'border-status-up/30',
        DEGRADED: 'border-status-degraded/30',
        DOWN: 'border-status-down/50',
        MAINTENANCE: 'border-status-maintenance/30',
        PENDING: 'border-noc-border',
    }[status];
}
