import { useEffect, useId, useRef } from 'react';
import Button from '@/components/shared/Button';

/**
 * Centered confirmation modal for destructive actions.
 *
 * Same props as the dialog it replaces, plus a tone switch for the confirm
 * Escape and overlay click close it. Focus moves to Cancel on open and
 * returns to the trigger on close, Tab stays inside the panel, and body
 * scroll locks while open. Cancel stays enabled while pending. Confirm
 * fires once through the loading state; the caller closes on success.
 */
export default function ConfirmModal({
    open,
    title,
    message,
    confirmLabel,
    onConfirm,
    onClose,
    pending = false,
    tone = 'danger',
    error,
}: {
    open: boolean;
    title: string;
    message: string;
    confirmLabel: string;
    onConfirm: () => void;
    onClose: () => void;
    pending?: boolean;
    tone?: 'danger' | 'info';
    /** Failure text from the caller; the panel stays open and names what went wrong. */
    error?: string;
}) {
    const panelRef = useRef<HTMLDivElement>(null);
    const restoreFocusRef = useRef<HTMLElement | null>(null);
    const titleId = useId();
    const messageId = useId();

    useEffect(() => {
        if (!open) return;

        restoreFocusRef.current = document.activeElement as HTMLElement | null;

        const focusables = () =>
            Array.from(
                panelRef.current?.querySelectorAll<HTMLElement>(
                    'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
                ) ?? [],
            );

        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                onClose();
                return;
            }

            if (e.key !== 'Tab') return;

            const items = focusables();
            if (items.length === 0) return;

            const first = items[0];
            const last = items[items.length - 1];

            if (e.shiftKey && document.activeElement === first) {
                e.preventDefault();
                last.focus();
            } else if (!e.shiftKey && document.activeElement === last) {
                e.preventDefault();
                first.focus();
            }
        };

        document.addEventListener('keydown', onKey);
        document.body.style.overflow = 'hidden';

        // Move focus to Cancel (first focusable) on open
        const timer = window.setTimeout(() => focusables()[0]?.focus(), 50);

        return () => {
            document.removeEventListener('keydown', onKey);
            document.body.style.overflow = '';
            window.clearTimeout(timer);
            restoreFocusRef.current?.focus?.();
        };
    }, [open, onClose]);

    if (!open) return null;

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
            <div className="absolute inset-0 bg-black/70" onClick={onClose} aria-hidden />
            <div
                ref={panelRef}
                role="alertdialog"
                aria-modal="true"
                aria-labelledby={titleId}
                aria-describedby={messageId}
                className="relative w-full max-w-sm rounded-md border border-noc-border bg-noc-surface p-5 shadow-lift"
            >
                <h2 id={titleId} className="font-display text-base font-semibold text-noc-text">
                    {title}
                </h2>
                <p id={messageId} className="mt-1 text-sm text-noc-muted">
                    {message}
                </p>
                {error && (
                    <p role="alert" className="mt-2 text-xs text-status-down">
                        {error}
                    </p>
                )}
                <div className="mt-5 flex justify-end gap-2">
                    <Button variant="ghost" onClick={onClose}>
                        Cancel
                    </Button>
                    <Button
                        variant={tone === 'danger' ? 'danger' : 'primary'}
                        loading={pending}
                        onClick={onConfirm}
                    >
                        {confirmLabel}
                    </Button>
                </div>
            </div>
        </div>
    );
}
