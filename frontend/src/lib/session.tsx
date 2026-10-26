import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';

export interface AuthUser {
    id: string;
    name: string;
    email: string;
    role: string;
}

interface MeResponse {
    user: AuthUser | null;
    // Optional while the API build may omit it; the flag only unlocks Users.
    can?: { manageUsers: boolean };
}

export interface Session {
    user: AuthUser | null;
    can: { manageUsers: boolean };
    /** The session check has not settled yet. */
    isLoading: boolean;
    /** The API says there is no usable session: no user, or 401 on /me. */
    isLoggedOut: boolean;
    /** A settled failure that is not a logout (backend down, 500): session unknown. */
    error: unknown;
    refetch: () => void;
}

/**
 * The signed-in account, loaded once from GET /me and shared through the query
 * cache (session user plus the can.manageUsers flag).
 *
 * Two responses mean "logged out": 200 with `user: null` (no session at all)
 * and 401 (session existed but was revoked or the account was disabled).
 */
export function useSession(): Session {
    const query = useQuery({
        queryKey: queryKeys.session,
        queryFn: async (): Promise<MeResponse> => {
            const { data } = await api.get<MeResponse>('/me');
            return data;
        },
        // A definitive answer (401/403/500) is not retried; only a transport
        // failure gets one more attempt before the screen reports it.
        retry: (failureCount, error) =>
            !axios.isAxiosError(error) || error.response === undefined ? failureCount < 1 : false,
        staleTime: 30_000,
    });

    const user = query.data?.user ?? null;
    // A 401 on /me (revoked session, disabled account) and a 200 that carries no
    // user both mean logged out, even when an earlier answer is still cached.
    const unauthorized =
        axios.isAxiosError(query.error) && query.error.response?.status === 401;
    const isLoggedOut = unauthorized || query.data?.user === null;

    return {
        user,
        can: { manageUsers: query.data?.can?.manageUsers ?? false },
        isLoading: query.isPending,
        isLoggedOut,
        error: query.error !== null && !unauthorized ? query.error : null,
        refetch: () => {
            void query.refetch();
        },
    };
}
