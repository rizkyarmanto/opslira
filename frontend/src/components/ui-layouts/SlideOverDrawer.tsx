import { useEffect, useRef, type ReactNode } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { X } from 'lucide-react';

/**
 * Slide-over panel for the target form.
 *
 * Behaves like a dialog: role and aria-modal for screen readers, Escape closes it,
 * focus moves in on open and returns to the trigger on close, and Tab is trapped
 * inside while it is open (R-32). Body scroll is locked so the page behind cannot
 * scroll away under the panel.
 */
export default function SlideOverDrawer({
    open,
    onClose,
    title,
    description,
    children,
    footer,
}: {
    open: boolean;
    onClose: () => void;
    title: string;
    description?: string;
    children: ReactNode;
    footer?: ReactNode;
}) {
    const panelRef = useRef<HTMLDivElement>(null);
    const restoreFocusRef = useRef<HTMLElement | null>(null);

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

        // Move focus into the panel on open
        const timer = window.setTimeout(() => focusables()[0]?.focus(), 50);

        return () => {
            document.removeEventListener('keydown', onKey);
            document.body.style.overflow = '';
            window.clearTimeout(timer);
            restoreFocusRef.current?.focus?.();
        };
    }, [open, onClose]);

    return (
        <AnimatePresence>
            {open && (
                <div className="fixed inset-0 z-50">
                    <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        onClick={onClose}
                        className="absolute inset-0 bg-black/70"
                    />
                    <motion.div
                        ref={panelRef}
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="drawer-title"
                        aria-describedby={description ? 'drawer-description' : undefined}
                        initial={{ x: '100%' }}
                        animate={{ x: 0 }}
                        exit={{ x: '100%' }}
                        transition={{ type: 'spring', stiffness: 340, damping: 36 }}
                        className="absolute right-0 top-0 flex h-full w-full max-w-xl flex-col border-l border-noc-border bg-noc-bg shadow-lift"
                    >
                        <div className="flex items-start justify-between gap-4 border-b border-noc-border px-6 py-4">
                            <div>
                                <h2 id="drawer-title" className="font-display text-lg font-semibold">
                                    {title}
                                </h2>
                                {description && (
                                    <p id="drawer-description" className="mt-0.5 text-xs text-noc-muted">
                                        {description}
                                    </p>
                                )}
                            </div>
                            <button
                                onClick={onClose}
                                aria-label="Close panel"
                                className="tap-target inline-flex items-center justify-center rounded-sm text-noc-muted hover:bg-white/5 hover:text-noc-text"
                            >
                                <X size={18} />
                            </button>
                        </div>
                        <div className="flex-1 overflow-y-auto px-6 py-5">{children}</div>
                        {footer && <div className="border-t border-noc-border px-6 py-4">{footer}</div>}
                    </motion.div>
                </div>
            )}
        </AnimatePresence>
    );
}
