import axios from 'axios';

/**
 * Axios instance for internal API calls (/api/v1/*).
 * CSRF token read from the meta tag rendered by the Inertia root view.
 */
export const api = axios.create({
    baseURL: '/api/v1',
    headers: {
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'application/json',
    },
});

api.interceptors.request.use((config) => {
    const token = document
        .querySelector('meta[name="csrf-token"]')
        ?.getAttribute('content');

    if (token) {
        config.headers['X-CSRF-TOKEN'] = token;
    }

    return config;
});

/**
 * Pull a human message out of a failed request.
 *
 * The response body comes from outside the app, so its shape is checked at runtime
 * rather than asserted: only a string `message` field is trusted, and anything else
 * falls back to the caller's default.
 */
export function apiErrorMessage(error: unknown, fallback: string): string {
    if (!axios.isAxiosError(error)) return fallback;

    const body: unknown = error.response?.data;
    if (body && typeof body === 'object' && 'message' in body) {
        const { message } = body;
        if (typeof message === 'string' && message.trim() !== '') return message;
    }

    return fallback;
}

