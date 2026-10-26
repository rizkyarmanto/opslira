import { useEffect, useMemo, useRef, useState } from 'react';
import { AnimatePresence, motion } from 'framer-motion';
import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { Search } from 'lucide-react';
import { api } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import StatusBadge from '@/components/shared/StatusBadge';
import ProtocolBadge from '@/components/beautiful-ui/ProtocolBadge';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import type { MonitoringTarget } from '@/types/monitoring';

const LISTBOX_ID = 'palette-target-listbox';

/**
 * Quick jump to a target by name, endpoint, or type (Ctrl/Cmd + K).
 *
 * This searches the real target list loaded from the API and navigates to the
 * selected target, so the control does what it says (R-26). Keyboard: arrows move,
 * Home/End jump to the ends, Enter opens, Escape closes (R-32). The combobox/listbox
 * roles and aria-activedescendant let a screen reader follow the same cursor the
 * keyboard drives (R-32).
 */
export default function CommandPalette() {
    const navigate = useNavigate();
    const [open, setOpen] = useState(false);
    const [query, setQuery] = useState('');
    const [debounced, setDebounced] = useState('');
    const [cursor, setCursor] = useState(0);
    const restoreFocusRef = useRef<HTMLElement | null>(null);

    // Debounced so a fast typist does not fire one request per keystroke.
    useEffect(() => {
        const id = window.setTimeout(() => setDebounced(query.trim()), 150);
        return () => window.clearTimeout(id);
    }, [query]);

    const { data, isPending, isError, error, refetch } = useQuery({
        queryKey: queryKeys.targets.search(debounced),
        queryFn: async () => {
            const { data } = await api.get<{ data: MonitoringTarget[] }>('/targets/search', {
                params: debounced ? { q: debounced } : {},
            });
            return data.data;
        },
        enabled: open,
        staleTime: 30_000,
    });

    const results = useMemo(() => data ?? [], [data]);
    const settled = open && !isPending && !isError;

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
            if (e.key === 'Home') {
                e.preventDefault();
                setCursor(0);
            }
            if (e.key === 'End') {
                e.preventDefault();
                setCursor(Math.max(results.length - 1, 0));
            }
            if (e.key === 'Enter' && results[cursor]) {
                e.preventDefault();
                setOpen(false);
                navigate(`/targets/${results[cursor].id}`);
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
                aria-haspopup="dialog"
                aria-expanded={open}
                aria-label="Search targets"
                className="hidden w-full max-w-md items-center gap-2 rounded-sm border border-noc-border bg-white/5 px-3 py-2 text-sm text-noc-muted transition-colors hover:border-zinc-600 hover:text-noc-text md:flex"
            >
                <Search size={15} aria-hidden />
                <span>Search target, IP, or domain</span>
                <kbd className="ml-auto rounded-xs border border-noc-border px-1.5 py-0.5 text-[10px] text-noc-muted">
                    Ctrl K
                </kbd>
            </button>

            {/* Below md the header runs out of room, so the trigger collapses to its
                icon inside the standard 44px tap target. */}
            <button
                onClick={() => setOpen(true)}
                aria-haspopup="dialog"
                aria-expanded={open}
                aria-label="Search targets"
                className="tap-target inline-flex items-center justify-center rounded-sm border border-noc-border bg-white/5 text-noc-muted transition-colors hover:border-zinc-600 hover:text-noc-text md:hidden"
            >
                <Search size={16} aria-hidden />
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
                                    role="combobox"
                                    aria-expanded={open}
                                    aria-controls={LISTBOX_ID}
                                    aria-activedescendant={results[cursor] ? `palette-option-${results[cursor].id}` : undefined}
                                    className="w-full bg-transparent text-sm text-noc-text placeholder:text-zinc-500"
                                />
                            </div>

                            <p aria-live="polite" className="sr-only">
                                {settled ? `${results.length} results` : ''}
                            </p>

                            <div className="max-h-72 overflow-y-auto p-2">
                                {isPending && (
                                    <div className="flex justify-center px-3 py-6">
                                        <ReloadIndicator label="Searching" />
                                    </div>
                                )}
                                {isError && (
                                    <div className="px-3 py-6 text-center">
                                        <p className="text-xs text-status-down">
                                            Failed to load targets: {(error as Error).message}
                                        </p>
                                        <button
                                            type="button"
                                            onClick={() => void refetch()}
                                            className="mt-3 rounded-sm border border-noc-border px-3 py-1.5 text-xs text-noc-text hover:bg-white/5"
                                        >
                                            Try again
                                        </button>
                                    </div>
                                )}
                                {settled && results.length === 0 && (
                                    <div className="px-3 py-6 text-center">
                                        <p className="text-xs text-noc-muted">
                                            {debounced
                                                ? `No targets match "${debounced}". Try a host, an IP, or a type such as INTERNET.`
                                                : 'No targets registered yet.'}
                                        </p>
                                        {!debounced && (
                                            <Link to="/targets" className="mt-3 inline-block text-xs text-noc-accent hover:underline">
                                                Open the Targets page
                                            </Link>
                                        )}
                                    </div>
                                )}
                                <div id={LISTBOX_ID} role="listbox" aria-label="Target results">
                                    {results.map((target, i) => (
                                        <button
                                            key={target.id}
                                            id={`palette-option-${target.id}`}
                                            role="option"
                                            aria-selected={i === cursor}
                                            onMouseEnter={() => setCursor(i)}
                                            onClick={() => {
                                                setOpen(false);
                                                navigate(`/targets/${target.id}`);
                                            }}
                                            className={`flex w-full items-center gap-3 rounded-sm px-3 py-2 text-left ${
                                                i === cursor ? 'bg-white/5' : ''
                                            }`}
                                        >
                                            <div className="min-w-0 flex-1">
                                                <p className="truncate text-sm text-noc-text">{target.name}</p>
                                                <p className="tabular truncate text-xs text-noc-muted">
                                                    {target.host_or_endpoint}
                                                    {target.port ? `:${target.port}` : ''}
                                                </p>
                                            </div>
                                            <ProtocolBadge protocolType={target.protocol_type} />
                                            <StatusBadge status={target.current_status} />
                                            {i === cursor && (
                                                <kbd className="rounded-xs border border-noc-border px-1.5 py-0.5 text-[10px] text-noc-muted">
                                                    Enter
                                                </kbd>
                                            )}
                                        </button>
                                    ))}
                                </div>
                            </div>
                        </motion.div>
                    </div>
                )}
            </AnimatePresence>
        </>
    );
}
