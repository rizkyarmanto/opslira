/**
 * Placeholder for a metric the probe has not produced yet.
 *
 * Renders a muted "n/a" rather than a dash glyph: a dash reads as punctuation, while
 * "n/a" says the value is genuinely absent, and screen readers announce it as such
 * instead of reading a symbol name (R-17, R-38).
 */
export default function NoData({ className = '' }: { className?: string }) {
    return (
        <span className={`text-noc-muted ${className}`} aria-label="No data">
            n/a
        </span>
    );
}
