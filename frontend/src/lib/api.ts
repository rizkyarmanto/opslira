import axios from 'axios';

/**
 * Axios instance for Go API calls (/api/v1/*).
 *
 * Base URL comes from VITE_API_BASE_URL and defaults to the local Go API.
 * withXSRFToken stays on: the Go API sets an XSRF-TOKEN cookie and checks
 * the X-XSRF-TOKEN header on state-changing requests.
 */
export const api = axios.create({
    baseURL: `${import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8080/api/v1'}`,
    // The API is served from another origin (dev :8080, compose localhost:8080),
    // so opslira_session and XSRF-TOKEN only ride requests when axios may send
    // credentials. The API's CORS answers one origin with credentials allowed,
    // and without this flag every request would arrive unauthenticated.
    withCredentials: true,
    withXSRFToken: true,
    headers: {
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'application/json',
    },
});

api.interceptors.response.use(
    (res) => res,
    (error) => {
        if (
            axios.isAxiosError(error) &&
            error.response?.status === 401 &&
            typeof window !== 'undefined' &&
            !window.location.pathname.startsWith('/login')
        ) {
            window.location.assign('/login');
        }
        return Promise.reject(error);
    },
);

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

/**
 * Absolute URL for file downloads, which go through a plain anchor rather
 * than axios so the browser handles the stream.
 */
export function apiUrl(path: string): string {
    const base = `${import.meta.env.VITE_API_BASE_URL ?? 'http://127.0.0.1:8080/api/v1'}`;
    return `${base.replace(/\/$/, '')}/${path.replace(/^\//, '')}`;
}

/**
 * Field errors from a 422 response: { errors: { field: [message] } }.
 * Only the first message per field is kept.
 */
export function apiFieldErrors(error: unknown): Record<string, string> {
    if (!axios.isAxiosError(error)) return {};
    const body: unknown = error.response?.data;
    if (!body || typeof body !== 'object' || !('errors' in body)) return {};
    const { errors } = body as { errors: unknown };
    if (!errors || typeof errors !== 'object') return {};
    const out: Record<string, string> = {};
    for (const [field, messages] of Object.entries(errors as Record<string, unknown>)) {
        if (Array.isArray(messages)) {
            const first = messages.find((m): m is string => typeof m === 'string' && m.trim() !== '');
            if (first) out[field] = first;
        } else if (typeof messages === 'string' && messages.trim() !== '') {
            out[field] = messages;
        }
    }
    return out;
}
