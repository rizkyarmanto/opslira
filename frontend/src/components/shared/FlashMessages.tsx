import { useFlash } from '@/lib/flash';

/**
 * Feedback from an action that has no result area of its own, rendered in one
 * place so every action that redirects can report its result (R-26, R-35).
 */
export default function FlashMessages() {
    const { flash } = useFlash();

    const success = flash?.success;
    const error = flash?.error;
    const message = flash?.message;

    if (!success && !error && !message) return null;

    return (
        <div className="mb-4 space-y-2">
            {success && (
                <p role="status" className="rounded-sm border border-status-up/30 bg-status-up/10 px-3 py-2 text-sm text-status-up">
                    {success}
                </p>
            )}
            {error && (
                <p role="alert" className="rounded-sm border border-status-down/40 bg-status-down/10 px-3 py-2 text-sm text-status-down">
                    {error}
                </p>
            )}
            {message && (
                <p role="status" className="rounded-sm border border-noc-border bg-white/5 px-3 py-2 text-sm text-noc-text">
                    {message}
                </p>
            )}
        </div>
    );
}
