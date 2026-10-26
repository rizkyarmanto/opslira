import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useSession } from '@/lib/session';
import { LoadingState } from '@/routes/QueryState';

/**
 * Guest only. A signed-in visitor is sent to the NOC overview, matching the
 * guest middleware the sign-in screen had before: you cannot open the form
 * over a live session.
 */
export function RequireGuest() {
    const { user, isLoading } = useSession();
    const location = useLocation();

    if (isLoading) {
        return (
            <div className="mx-auto w-full max-w-[1600px] px-4 py-16 md:px-6">
                <LoadingState label="Checking session" />
            </div>
        );
    }

    if (user) {
        return <Navigate to="/dashboard/noc" replace state={{ from: location.pathname }} />;
    }

    return <Outlet />;
}
