import { useEffect } from 'react';

const appName = import.meta.env.VITE_APP_NAME || 'Opslira';

/**
 * Sets the document title the way the app shell did when the server rendered
 * each page: "Screen name | Opslira", or just the app name for a screen
 * without one.
 */
export function Head({ title }: { title?: string }) {
    useEffect(() => {
        document.title = title ? `${title} | ${appName}` : appName;
    }, [title]);

    return null;
}
