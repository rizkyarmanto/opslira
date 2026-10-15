import type { ReactNode } from 'react';
import type { TargetStatus } from '@/types/monitoring';
import { statusAccent } from '@/components/beautiful-ui/ProtocolBadge';
import { StatusRail } from '@/components/shared/StatusBadge';

/**
 * Target card: identity, status, and metrics in one scannable block.
 *
 * No looping border animation and no pulsing dot. A failing target is marked by the
 * status rail, the tinted border, and the badge, all of which are static and always
 * visible (DESIGN.md, R-19, R-31).
 *
 * Heartbeat bars are passed in as a status series. A null entry means "no check
 * recorded for that slot", rendered as an empty slot rather than a fabricated green
 * bar (R-17, R-38).
 */
export default function StatusCard({
    status,
    header,
    children,
    heartbeat,
    footer,
}: {
    status: TargetStatus;
    header: ReactNode;
    children?: ReactNode;
    heartbeat?: Array<'UP' | 'DEGRADED' | 'DOWN' | null>;
    footer?: ReactNode;
}) {
    const bars = heartbeat ?? [];

    return (
        <div
            className={`relative overflow-hidden rounded-md border bg-noc-surface p-5 ${statusAccent(status)}`}
        >
            <StatusRail status={status} />

            <div className="pl-2">
                <div className="mb-3 flex items-start justify-between gap-3">
                    <div className="min-w-0">{header}</div>
                </div>

                {children}

                {bars.length > 0 && (
                    <div className="mt-4">
                        <div className="mb-1.5 flex items-baseline justify-between">
                            <span className="field-label">Check history</span>
                            <span className="text-[10px] text-noc-muted">{bars.length} latest</span>
                        </div>
                        <div className="flex h-6 items-end gap-px">
                            {bars.map((bar, i) => (
                                <span
                                    key={i}
                                    title={bar ?? 'No data'}
                                    className={`h-full flex-1 rounded-xs ${
                                        bar === 'UP'
                                            ? 'bg-status-up/70'
                                            : bar === 'DEGRADED'
                                              ? 'bg-status-degraded/70'
                                              : bar === 'DOWN'
                                                ? 'bg-status-down/80'
                                                : 'bg-white/5'
                                    }`}
                                />
                            ))}
                        </div>
                    </div>
                )}

                {footer && <div className="mt-4 border-t border-noc-border pt-3">{footer}</div>}
            </div>
        </div>
    );
}
