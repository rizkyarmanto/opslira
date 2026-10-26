import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { LogOut } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Card from '@/components/shared/Card';
import Button from '@/components/shared/Button';
import { api, apiErrorMessage } from '@/lib/api';
import { Head } from '@/lib/head';
import { useFlash } from '@/lib/flash';
import { queryKeys } from '@/lib/query-keys';
import { useSession } from '@/lib/session';

/**
 * Settings. Session controls and the account currently signed in, both read from
 * the signed-in session rather than hardcoded copy.
 */
export default function Settings() {
    const { user } = useSession();
    const { setFlash } = useFlash();
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [signingOut, setSigningOut] = useState(false);

    const signOut = async () => {
        setSigningOut(true);
        try {
            await api.post('/auth/logout');
            queryClient.removeQueries({ queryKey: queryKeys.session });
            navigate('/login', { replace: true });
        } catch (error) {
            setFlash({ error: apiErrorMessage(error, 'Could not sign out. Please try again.') });
        } finally {
            setSigningOut(false);
        }
    };

    return (
        <AppLayout title="Settings">
            <Head title="Settings" />

            <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
                <Card className="p-5">
                    <h2 className="mb-3 text-sm font-semibold text-noc-text">Account</h2>
                    {user ? (
                        <dl className="space-y-2 text-sm">
                            <div className="flex items-baseline justify-between gap-3">
                                <dt className="field-label">Name</dt>
                                <dd className="text-noc-text">{user.name}</dd>
                            </div>
                            <div className="flex items-baseline justify-between gap-3">
                                <dt className="field-label">Email</dt>
                                <dd className="tabular text-noc-text">{user.email}</dd>
                            </div>
                            <div className="flex items-baseline justify-between gap-3">
                                <dt className="field-label">Role</dt>
                                <dd className="text-noc-text">{user.role.replace(/_/g, ' ')}</dd>
                            </div>
                        </dl>
                    ) : (
                        <p className="text-sm text-noc-muted">No active session.</p>
                    )}
                </Card>

                <Card className="p-5">
                    <h2 className="mb-3 text-sm font-semibold text-noc-text">Session</h2>
                    <p className="mb-4 text-xs text-noc-muted">
                        Signing out will end this session. You will need to sign in again to open the dashboard.
                    </p>
                    <Button variant="danger" loading={signingOut} onClick={() => void signOut()}>
                        <LogOut size={15} aria-hidden />
                        Sign out
                    </Button>
                </Card>

                <Card className="p-5 lg:col-span-2">
                    <h2 className="mb-3 text-sm font-semibold text-noc-text">About Opslira</h2>
                    <p className="text-xs leading-relaxed text-noc-muted">
                        Opslira monitors seven target categories in the office environment: network devices, web
                        applications, domains and TLS certificates, databases, mail servers, S3 object storage, and
                        servers reported by Beszel. Each target is checked by an adapter according to its protocol,
                        and its status is used for reports and incident notifications.
                    </p>
                </Card>
            </div>
        </AppLayout>
    );
}
