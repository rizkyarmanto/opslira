/**
 * Human relative time for "last checked" labels.
 * Returns null when there is no timestamp, so callers render an honest empty state
 * instead of "Invalid Date" (R-17).
 */
export function relativeTime(iso: string | null | undefined): string | null {
    if (!iso) return null;

    const then = new Date(iso).getTime();
    if (Number.isNaN(then)) return null;

    const seconds = Math.round((Date.now() - then) / 1000);

    if (seconds < 0) return 'just now';
    if (seconds < 60) return `${seconds} seconds ago`;

    const minutes = Math.round(seconds / 60);
    if (minutes < 60) return `${minutes} minutes ago`;

    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours} hours ago`;

    const days = Math.round(hours / 24);
    return `${days} days ago`;
}

export function formatDateTime(iso: string | null | undefined): string | null {
    if (!iso) return null;
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return null;
    return d.toLocaleString('en-US', { dateStyle: 'medium', timeStyle: 'short' });
}
