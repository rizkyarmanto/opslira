import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useSession } from '@/lib/session';
import { LoadingState } from '@/routes/QueryState';

/**
 * Signed-in shell. Unknown session shows a loader, logged-out goes to
 * sign in, settled failures report instead of bouncing to login in a loop.
 */
export function RequireAuth() {
    const { user, isLoading, isLoggedOut, error } = useSession();
    const location = useLocation();

    if (isLoading) {
        return (
            <div className="mx-auto w-full max-w-[1600px] px-4 py-16 md:px-6">
                <LoadingState label="Checking session" />
            </div>
        );
    }

    if (isLoggedOut || !user) {
        return <Navigate to="/login" replace state={{ from: location.pathname }} />;
    }

    if (error) {
        return (
            <div className="mx-auto w-full max-w-[1600px] px-4 py-16 md:px-6">
                <p role="alert" className="rounded-md border border-status-down/40 bg-status-down/10 px-4 py-6 text-center text-sm text-status-down">
                    Could not verify the session. Check the API connection and reload the page.
                </p>
            </div>
        );
    }

    return <Outlet />;
}
