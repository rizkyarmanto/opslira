import '../css/app.css';

import { createRoot } from 'react-dom/client';
import { createInertiaApp } from '@inertiajs/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { resolvePageComponent } from 'laravel-vite-plugin/inertia-helpers';

const appName = import.meta.env.VITE_APP_NAME || 'Opslira';

// TanStack Query client for dashboard polling.
// refetchIntervalInBackground defaults false => Page Visibility Throttling
// (docs/referensi-design.md §8.2)
const queryClient = new QueryClient({
    defaultOptions: {
        queries: {
            refetchIntervalInBackground: false,
            refetchOnWindowFocus: true,
            staleTime: 10_000,
            retry: 1,
        },
    },
});

createInertiaApp({
    title: (title) => (title ? `${title} | ${appName}` : appName),
    resolve: (name) =>
        resolvePageComponent(
            `./pages/${name}.tsx`,
            import.meta.glob('./pages/**/*.tsx'),
        ),
    setup({ el, App, props }) {
        createRoot(el).render(
            <QueryClientProvider client={queryClient}>
                <App {...props} />
            </QueryClientProvider>,
        );
    },
    progress: {
        color: '#9fe870',
    },
});
