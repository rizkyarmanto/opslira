import { useEffect, useState } from 'react';
import { router } from '@inertiajs/react';

/**
 * Polls the current Inertia page props on an interval so dashboard status stays
 * current (docs/referensi-design.md §8.2).
 *
 * Throttling: the timer pauses while the tab is hidden and fires immediately when it
 * becomes visible again, so a background tab does not hammer the server.
 *
 * Returns the timestamp of the last successful refresh so the UI can show a real
 * "diperbarui ..." label instead of a fabricated one (R-38), plus `refreshing`
 * while a reload is in flight so the UI can show the lattice loader instead of
 * claiming the data is current.
 */
export function usePolling(only: string[], intervalMs = 15_000) {
    // The server just rendered these props, so the data is fresh as of mount. Showing
    // "memuat" until the first poll would understate what is already on screen.
    const [lastUpdated, setLastUpdated] = useState<Date | null>(() => new Date());
    const [refreshing, setRefreshing] = useState(false);

    useEffect(() => {
        let timer: number | undefined;

        const tick = () => {
            // onStart/onFinish bracket the whole reload: onSuccess fires before the
            // response is applied, so the timestamp must move on finish instead.
            router.reload({
                only,
                onStart: () => setRefreshing(true),
                onFinish: () => {
                    setRefreshing(false);
                    setLastUpdated(new Date());
                },
            });
        };

        const start = () => {
            window.clearInterval(timer);
            timer = window.setInterval(tick, intervalMs);
        };

        const stop = () => window.clearInterval(timer);

        const onVisibility = () => {
            if (document.hidden) {
                stop();
            } else {
                tick();
                start();
            }
        };

        start();
        document.addEventListener('visibilitychange', onVisibility);

        return () => {
            stop();
            document.removeEventListener('visibilitychange', onVisibility);
        };
        // `only` is a stable array literal per page; join keeps the effect from
        // re-subscribing on every render.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [only.join(','), intervalMs]);

    return { lastUpdated, refreshing };
}
