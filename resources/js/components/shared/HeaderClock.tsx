import { useEffect, useState } from 'react';

const timeFormatter = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'Asia/Jakarta',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
});

const fullFormatter = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'Asia/Jakarta',
    dateStyle: 'full',
    timeStyle: 'long',
});

/**
 * Live office clock in the header, always Asia/Jakarta so the NOC reads the
 * same wall time as APP_TIMEZONE no matter which machine the browser runs on.
 *
 * Returns null until mounted so first paint never flashes a placeholder time,
 * and carries no aria-live: a per-second announcement would be noise.
 */
export default function HeaderClock() {
    const [now, setNow] = useState<Date | null>(null);

    useEffect(() => {
        const tick = () => setNow(new Date());
        tick();
        const id = window.setInterval(tick, 1000);

        return () => window.clearInterval(id);
    }, []);

    if (now === null) {
        return null;
    }

    return (
        <time
            dateTime={now.toISOString()}
            className="tabular text-xs text-noc-muted"
            title={fullFormatter.format(now)}
        >
            {timeFormatter.format(now)} WIB
        </time>
    );
}
