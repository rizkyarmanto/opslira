import type { ReactNode } from 'react';
import { AlertTriangle, CheckCircle2, CircleDashed, MinusCircle, XOctagon } from 'lucide-react';
import type { TargetStatus } from '@/types/monitoring';

/**
 * Status tokens. Icon and label are chosen for what they say about the target:
 * a check for healthy, a triangle for degraded, an octagon for down, a pause bar
 * for planned maintenance, a dashed circle for "not yet probed".
 * Color is a third carrier, never the only one (DESIGN.md, R-25, R-31).
 */
const config: Record<
    TargetStatus,
    { classes: string; icon: ReactNode; label: string; dot: string }
> = {
    UP: {
        classes: 'status-up',
        icon: <CheckCircle2 size={13} aria-hidden />,
        label: 'UP',
        dot: 'bg-status-up',
    },
    DEGRADED: {
        classes: 'status-degraded',
        icon: <AlertTriangle size={13} aria-hidden />,
        label: 'DEGRADED',
        dot: 'bg-status-degraded',
    },
    DOWN: {
        classes: 'status-down',
        icon: <XOctagon size={13} aria-hidden />,
        label: 'DOWN',
        dot: 'bg-status-down',
    },
    MAINTENANCE: {
        classes: 'status-maintenance',
        icon: <MinusCircle size={13} aria-hidden />,
        label: 'MAINTENANCE',
        dot: 'bg-status-maintenance',
    },
    PENDING: {
        classes: 'status-pending',
        icon: <CircleDashed size={13} aria-hidden />,
        label: 'NOT YET CHECKED',
        dot: 'bg-zinc-500',
    },
};

export default function StatusBadge({ status }: { status: TargetStatus }) {
    const { classes, icon, label } = config[status] ?? config.PENDING;

    return (
        <span
            className={`inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-semibold ${classes}`}
        >
            {icon}
            {label}
        </span>
    );
}

/**
 * The status rail: a tinted leading rule that carries status on rows and cards.
 * It is the one repeated gesture that makes a target row read the same on every
 * screen (DESIGN.md identity motif). It sits next to the badge, so it decorates
 * nothing on its own: it is a second, scannable copy of a real state.
 */
export function StatusRail({ status }: { status: TargetStatus }) {
    const { dot } = config[status] ?? config.PENDING;

    return <span aria-hidden className={`absolute inset-y-0 left-0 w-[3px] rounded-l-md ${dot}`} />;
}
