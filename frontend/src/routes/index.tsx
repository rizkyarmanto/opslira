import { createBrowserRouter, Navigate, Outlet } from 'react-router-dom';
import Login from '@/pages/Auth/Login';
import Settings from '@/pages/Settings';
import { FlashProvider } from '@/lib/flash';
import { RequireAuth } from '@/routes/RequireAuth';
import { RequireGuest } from '@/routes/RequireGuest';
import NocRoute from '@/routes/NocRoute';
import AlertsRoute from '@/routes/AlertsRoute';
import NetworkRoute from '@/routes/NetworkRoute';
import AppsRoute from '@/routes/AppsRoute';
import MailRoute from '@/routes/MailRoute';
import TargetsRoute from '@/routes/TargetsRoute';
import TargetDetailRoute from '@/routes/TargetDetailRoute';
import SchedulesRoute from '@/routes/SchedulesRoute';
import EmailLogsRoute from '@/routes/EmailLogsRoute';
import UsersRoute from '@/routes/UsersRoute';

function FlashShell() {
    return (
        <FlashProvider>
            <Outlet />
        </FlashProvider>
    );
}
export const router = createBrowserRouter([
    {
        element: <FlashShell />,
        children: [
    {
        element: <RequireGuest />,
        children: [{ path: '/login', element: <Login /> }],
    },
    {
        element: <RequireAuth />,
        children: [
            { path: '/', element: <Navigate to="/dashboard/noc" replace /> },
            { path: '/dashboard/noc', element: <NocRoute /> },
            { path: '/dashboard/alerts', element: <AlertsRoute /> },
            { path: '/dashboard/network', element: <NetworkRoute /> },
            { path: '/dashboard/apps-domains-db', element: <AppsRoute /> },
            { path: '/dashboard/mail-s3-beszel', element: <MailRoute /> },
            { path: '/targets', element: <TargetsRoute /> },
            { path: '/targets/:id', element: <TargetDetailRoute /> },
            { path: '/reports/schedules', element: <SchedulesRoute /> },
            { path: '/reports/email-logs', element: <EmailLogsRoute /> },
            { path: '/users', element: <UsersRoute /> },
            { path: '/settings', element: <Settings /> },
        ],
    },
    { path: '*', element: <Navigate to="/dashboard/noc" replace /> },
        ],
    },
]);
