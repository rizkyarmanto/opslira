import { AnimatePresence, motion } from 'framer-motion';
import type { ReactNode } from 'react';

/**
 * Smooth height + fade transition for dynamic protocol form fields.
 * Adapted from motion-primitives.com (docs/referensi-design.md §4.2)
 */
export default function TransitionPanel({
    panelKey,
    children,
}: {
    panelKey: string;
    children: ReactNode;
}) {
    return (
        <AnimatePresence mode="wait" initial={false}>
            <motion.div
                key={panelKey}
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: 'auto', opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.25, ease: 'easeInOut' }}
                className="overflow-hidden"
            >
                {children}
            </motion.div>
        </AnimatePresence>
    );
}
