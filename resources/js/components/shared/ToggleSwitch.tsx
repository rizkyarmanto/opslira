/**
 * State toggle for maintenance mode and schedule activation.
 *
 * The visible control is 24px tall, so the hit area is expanded to a 44px tap target
 * (R-03). The focus ring comes from the global :focus-visible rule (R-32).
 *
 * `tone` picks the "on" color by meaning: `accent` for an ordinary enabled state,
 * `maintenance` for planned maintenance, so the two never read as the same thing
 * (R-31).
 */
export default function ToggleSwitch({
    checked,
    onChange,
    disabled = false,
    label,
    tone = 'accent',
}: {
    checked: boolean;
    onChange: (next: boolean) => void;
    disabled?: boolean;
    label: string;
    tone?: 'accent' | 'maintenance';
}) {
    const onColor = tone === 'maintenance' ? 'bg-status-maintenance' : 'bg-noc-accent';

    return (
        <button
            type="button"
            role="switch"
            aria-checked={checked}
            aria-label={label}
            disabled={disabled}
            onClick={() => onChange(!checked)}
            className="tap-target inline-flex shrink-0 items-center justify-center disabled:cursor-not-allowed disabled:opacity-50"
        >
            <span
                className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    checked ? onColor : 'bg-white/10'
                }`}
            >
                <span
                    className={`inline-block h-5 w-5 transform rounded-full bg-white transition-transform ${
                        checked ? 'translate-x-[22px]' : 'translate-x-0.5'
                    }`}
                />
            </span>
        </button>
    );
}
