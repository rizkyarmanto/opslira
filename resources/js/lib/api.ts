import axios from 'axios';

/**
 * Axios instance for internal API calls (/api/v1/*).
 *
 * CSRF rides the encrypted XSRF-TOKEN cookie (withXSRFToken), which Laravel
 * refreshes on every response. The render-time meta token must NOT be sent:
 * login regenerates the session token, an SPA page navigation never re-renders
 * the meta tag, and a stale X-CSRF-TOKEN header shadows the fresh X-XSRF-TOKEN
 * header in Laravel's token lookup, which returns 419 on every POST.
 */
export const api = axios.create({
    baseURL: '/api/v1',
    withXSRFToken: true,
    headers: {
        'X-Requested-With': 'XMLHttpRequest',
        Accept: 'application/json',
    },
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

