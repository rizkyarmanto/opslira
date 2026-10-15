import { useEffect, useMemo, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useQuery } from '@tanstack/react-query';
import { router } from '@inertiajs/react';
import { Search } from 'lucide-react';
import { api } from '@/lib/api';
import StatusBadge from '@/components/shared/StatusBadge';
import type { MonitoringTarget } from '@/types/monitoring';

/**
 * Quick jump to a target by name, endpoint, or type (Ctrl/Cmd + K).
 *
 * This searches the real target list loaded from the API and navigates to the
 * selected target, so the control does what it says (R-26). Keyboard: arrows move,
 * Enter opens, Escape closes (R-32).
 */
export default function CommandPalette() {
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState('');
    const [cursor, setCursor] = useState(0);
    const restoreFocusRef = useRef<HTMLElement | null>(null);

    const { data, isPending, isError, error } = useQuery({
        queryKey: ['targets', 'search', query.trim()],
        queryFn: async () => {
            const { data } = await api.get<{ data: MonitoringTarget[] }>('/targets/search', {
                params: query.trim() ? { q: query.trim() } : {},
            });
            return data.data;
        },
        enabled: open,
        staleTime: 30_000,
    });

    const results = useMemo(() => data ?? [], [data]);

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
                e.preventDefault();
                setOpen((v) => !v);
            }
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, []);

    useEffect(() => {
        setCursor(0);
    }, [query, open]);

    useEffect(() => {
        if (!open) return;

        restoreFocusRef.current = document.activeElement as HTMLElement | null;

        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                setOpen(false);
                return;
            }
            if (e.key === 'ArrowDown') {
                e.preventDefault();
                setCursor((c) => Math.min(c + 1, Math.max(results.length - 1, 0)));
            }
            if (e.key === 'ArrowUp') {
                e.preventDefault();
                setCursor((c) => Math.max(c - 1, 0));
            }
            if (e.key === 'Enter' && results[cursor]) {
                e.preventDefault();
                setOpen(false);
                router.visit(`/targets/${results[cursor].id}`);
            }
        };

        document.addEventListener('keydown', onKey);
        return () => {
            document.removeEventListener('keydown', onKey);
            restoreFocusRef.current?.focus?.();
        };
    }, [open, results, cursor]);

    return (
        <>
            <button
                onClick={() => setOpen(true)}
                className="flex w-full max-w-md items-center gap-2 rounded-sm border border-noc-border bg-white/5 px-3 py-2 text-sm text-noc-muted transition-colors hover:border-zinc-600 hover:text-noc-text"
            >
                <Search size={15} aria-hidden />
                <span>Search target, IP, or domain</span>
                <kbd className="ml-auto rounded-xs border border-noc-border px-1.5 py-0.5 text-[10px] text-noc-muted">
                    Ctrl K
                </kbd>
            </button>

            <AnimatePresence>
                {open && (
                    <div className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]">
                        <motion.div
                            initial={{ opacity: 0 }}
                            animate={{ opacity: 1 }}
                            exit={{ opacity: 0 }}
                            onClick={() => setOpen(false)}
                            className="absolute inset-0 bg-black/70"
                        />
                        <motion.div
                            role="dialog"
                            aria-modal="true"
                            aria-label="Search target"
                            initial={{ opacity: 0, y: -8 }}
                            animate={{ opacity: 1, y: 0 }}
                            exit={{ opacity: 0, y: -8 }}
                            transition={{ duration: 0.15 }}
                            className="relative w-full max-w-lg overflow-hidden rounded-md border border-noc-border bg-noc-surface shadow-lift"
                        >
                            <div className="flex items-center gap-2 border-b border-noc-border px-4 py-3">
                                <Search size={16} className="text-noc-muted" aria-hidden />
                                <input
                                    autoFocus
                                    value={query}
                                    onChange={(e) => setQuery(e.target.value)}
                                    placeholder="Type a target name, IP, or domain"
                                    aria-label="Search keyword"
                                    className="w-full bg-transparent text-sm text-noc-text placeholder:text-zinc-500"
                                />
                            </div>
                            <div className="max-h-72 overflow-y-auto p-2">
                                {isPending && (
                                    <p className="px-3 py-6 text-center text-xs text-noc-muted">
                                        Loading target list...
                                    </p>
                                )}
                                {isError && (
                                    <p className="px-3 py-6 text-center text-xs text-status-down">
                                        Failed to load targets: {(error as Error).message}
                                    </p>
                                )}
                                {!isPending && !isError && results.length === 0 && (
                                    <p className="px-3 py-6 text-center text-xs text-noc-muted">
                                        {query.trim()
                                            ? `No targets match "${query}".`
                                            : 'No targets registered yet.'}
                                    </p>
                                )}
                                {results.map((target, i) => (
                                    <button
                                        key={target.id}
                                        onMouseEnter={() => setCursor(i)}
                                        onClick={() => {
                                            setOpen(false);
                                            router.visit(`/targets/${target.id}`);
                                        }}
                                        className={`flex w-full items-center gap-3 rounded-sm px-3 py-2 text-left ${
                                            i === cursor ? 'bg-white/5' : ''
                                        }`}
                                    >
                                        <div className="min-w-0 flex-1">
                                            <p className="truncate text-sm text-noc-text">{target.name}</p>
                                            <p className="tabular truncate text-xs text-noc-muted">
                                                {target.host_or_endpoint}
                                            </p>
                                        </div>
                                        <StatusBadge status={target.current_status} />
                                    </button>
                                ))}
                            </div>
                        </motion.div>
                    </div>
                )}
            </AnimatePresence>
        </>
    );
}
