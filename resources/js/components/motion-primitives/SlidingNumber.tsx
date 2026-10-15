import { useEffect, useRef, useState } from 'react';
import { animate } from 'framer-motion';
import NoData from '@/components/shared/NoData';

/**
 * Rolling number for a metric that just changed value.
 *
 * The animation is a transition between two real readings, so it is skipped on first
 * paint (there is no previous value to roll from) and skipped when the tab is hidden
 * (Page Visibility Throttling, docs/referensi-design.md §8.2). A null value renders
 * an em dash, never a zero (R-17).
 */
export default function SlidingNumber({
    value,
    decimals = 0,
    suffix = '',
    duration = 0.45,
    className = '',
}: {
    value: number | null | undefined;
    decimals?: number;
    suffix?: string;
    duration?: number;
    className?: string;
}) {
    const [display, setDisplay] = useState<number | null>(value ?? null);
    const previous = useRef<number | null>(value ?? null);

    useEffect(() => {
        if (value === null || value === undefined) {
            previous.current = null;
            setDisplay(null);
            return;
        }

        const from = previous.current;
        previous.current = value;

        // No previous reading (first paint) or a hidden tab: show the value directly.
        if (from === null || document.hidden) {
            setDisplay(value);
            return;
        }

        const controls = animate(from, value, {
            duration,
            ease: 'easeOut',
            onUpdate: (v) => setDisplay(v),
        });

        return () => controls.stop();
    }, [value, duration]);

    if (display === null || value === null || value === undefined) {
        return <NoData className={`tabular ${className}`} />;
    }

    return (
        <span className={`tabular ${className}`}>
            {display.toFixed(decimals)}
            {suffix}
        </span>
    );
}
