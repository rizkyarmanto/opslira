import { Link } from '@inertiajs/react';
import { motion } from 'framer-motion';

export interface TabItem {
    id: string;
    label: string;
    count?: number;
}

/**
 * Environment filter. Each tab is a real Inertia navigation, so it changes the data
 * on the page and works with the keyboard out of the box (R-26, R-32). The sliding
 * indicator marks where you are; it is a position cue, not an ambient loop (R-19).
 */
export default function AnimatedTabs({
    tabs,
    active,
    buildHref,
}: {
    tabs: TabItem[];
    active: string;
    buildHref: (id: string) => string;
}) {
    return (
        <nav
            aria-label="Filter environment"
            className="inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-md border border-noc-border bg-noc-surface p-1"
        >
            {tabs.map((tab) => {
                const isActive = tab.id === active;
                return (
                    <Link
                        key={tab.id}
                        href={buildHref(tab.id)}
                        preserveState
                        preserveScroll
                        aria-current={isActive ? 'page' : undefined}
                        className={`relative shrink-0 whitespace-nowrap rounded-sm px-3 py-1.5 text-sm font-medium transition-colors ${
                            isActive ? 'text-zinc-950' : 'text-noc-muted hover:text-noc-text'
                        }`}
                    >
                        {isActive && (
                            <motion.span
                                layoutId="environment-tab"
                                className="absolute inset-0 rounded-sm bg-noc-accent"
                                transition={{ type: 'spring', stiffness: 420, damping: 34 }}
                            />
                        )}
                        <span className="relative z-10">
                            {tab.label}
                            {typeof tab.count === 'number' && (
                                <span className="tabular ml-1.5 text-xs opacity-70">{tab.count}</span>
                            )}
                        </span>
                    </Link>
                );
            })}
        </nav>
    );
}
