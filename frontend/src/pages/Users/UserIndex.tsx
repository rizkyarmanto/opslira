import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, MinusCircle, Pencil, Plus, RefreshCw } from 'lucide-react';
import AppLayout from '@/components/shared/AppLayout';
import Button from '@/components/shared/Button';
import ConfirmModal from '@/components/shared/ConfirmModal';
import ToggleSwitch from '@/components/shared/ToggleSwitch';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';
import UserFormDrawer from '@/pages/Users/UserFormDrawer';
import { api, apiErrorMessage } from '@/lib/api';
import { Head } from '@/lib/head';
import { queryKeys } from '@/lib/query-keys';
import { roleLabels } from '@/lib/schemas/user';
import { useSession } from '@/lib/session';
import type { ManagedUser } from '@/types/monitoring';

interface Props {
    users: ManagedUser[];
    canManageUsers: boolean;
}

/**
 * User administration. Each row shows who the account is, the role that decides what
 * it can reach, and whether it is active, with the two actions an admin needs:
 * enable/disable and edit (R-20, R-26).
 */
export default function UserIndex({ users, canManageUsers }: Props) {
    const { user } = useSession();
    const queryClient = useQueryClient();
    const currentUserId = user?.id ?? null;

    const [drawerOpen, setDrawerOpen] = useState(false);
    const [editing, setEditing] = useState<ManagedUser | null>(null);
    const [busyId, setBusyId] = useState<string | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [reloading, setReloading] = useState(false);
    const [confirmUser, setConfirmUser] = useState<ManagedUser | null>(null);
    const [confirmAction, setConfirmAction] = useState<'disable' | 'toggle'>('disable');

    const reload = async () => {
        setReloading(true);
        try {
            await queryClient.invalidateQueries({ queryKey: queryKeys.users });
        } finally {
            setReloading(false);
        }
    };

    const openCreate = () => {
        setEditing(null);
        setDrawerOpen(true);
    };

    const openEdit = (user: ManagedUser) => {
        setEditing(user);
        setDrawerOpen(true);
    };

    const openConfirm = (user: ManagedUser, action: 'disable' | 'toggle') => {
        setConfirmAction(action);
        setConfirmUser(user);
    };

    const runSetActive = async (user: ManagedUser, next: boolean) => {
        setBusyId(user.id);
        setError(null);
        try {
            await api.put(`/users/${user.id}`, {
                name: user.name,
                email: user.email,
                role: user.role,
                is_active: next,
            });
            setConfirmUser(null);
            reload();
        } catch (err) {
            setError(apiErrorMessage(err, 'Could not update this user.'));
        } finally {
            setBusyId(null);
        }
    };

    const setActive = (user: ManagedUser, next: boolean) => {
        if (next) {
            void runSetActive(user, true);
        } else {
            openConfirm(user, 'toggle');
        }
    };

    const runDisable = async (user: ManagedUser) => {
        setBusyId(user.id);
        setError(null);
        try {
            await api.delete(`/users/${user.id}`);
            setConfirmUser(null);
            reload();
        } catch (err) {
            setError(apiErrorMessage(err, 'Could not disable this user.'));
        } finally {
            setBusyId(null);
        }
    };

    return (
        <AppLayout title="Users">
            <Head title="Users" />

            <div className="mb-5 flex flex-wrap items-center gap-3">
                <p className="text-sm text-noc-muted">
                    Every account that can sign in, and the role that decides what it can reach.
                </p>

                <Button
                    variant="ghost"
                    onClick={reload}
                    aria-label="Reload user list"
                >
                    {reloading ? <ReloadIndicator label="Reloading" /> : <RefreshCw size={15} aria-hidden />}
                    Reload
                </Button>

                <Button className="ml-auto" onClick={openCreate} disabled={!canManageUsers}>
                    <Plus size={15} aria-hidden />
                    Add user
                </Button>
            </div>

            {error && (
                <p role="alert" className="mb-4 rounded-sm border border-status-down/40 bg-status-down/10 px-3 py-2 text-xs text-status-down">
                    {error}
                </p>
            )}

            {users.length === 0 ? (
                <div className="rounded-md border border-dashed border-noc-border px-6 py-12 text-center">
                    <p className="text-sm text-noc-text">No user accounts yet.</p>
                    <p className="mx-auto mt-1 max-w-md text-xs text-noc-muted">
                        Add the first account to let someone sign in to Opslira.
                    </p>
                    <Button className="mt-4" onClick={openCreate} disabled={!canManageUsers}>
                        <Plus size={15} aria-hidden />
                        Add user
                    </Button>
                </div>
            ) : (
                <>
                    {reloading && (
                        <div className="mb-3">
                            <ReloadIndicator label="Refreshing user list" />
                        </div>
                    )}
                    <div
                        className={`overflow-x-auto rounded-md border border-noc-border${reloading ? ' pointer-events-none opacity-60' : ''}`}
                        aria-busy={reloading}
                    >
                    <table className="w-full min-w-[720px] text-left text-sm">
                        <thead className="bg-white/[0.03]">
                            <tr>
                                <th scope="col" className="field-label px-4 py-3">Name</th>
                                <th scope="col" className="field-label px-4 py-3">Email</th>
                                <th scope="col" className="field-label px-4 py-3">Role</th>
                                <th scope="col" className="field-label px-4 py-3">Status</th>
                                <th scope="col" className="field-label px-4 py-3 text-right">Action</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-noc-border">
                            {users.map((user) => {
                                const isSelf = user.id === currentUserId;
                                return (
                                    <tr key={user.id} className="hover:bg-white/[0.02]">
                                        <td className="px-4 py-3">
                                            <span className="font-medium text-noc-text">{user.name}</span>
                                            {isSelf && <span className="ml-2 text-[11px] text-noc-muted">You</span>}
                                        </td>
                                        <td className="tabular px-4 py-3 text-noc-muted">{user.email}</td>
                                        <td className="px-4 py-3 text-noc-muted">{roleLabels[user.role]}</td>
                                        <td className="px-4 py-3">
                                            {user.is_active ? (
                                                <span className="inline-flex items-center gap-1.5 text-xs text-status-up">
                                                    <CheckCircle2 size={14} aria-hidden />
                                                    Active
                                                </span>
                                            ) : (
                                                <span className="inline-flex items-center gap-1.5 text-xs text-noc-muted">
                                                    <MinusCircle size={14} aria-hidden />
                                                    Disabled
                                                </span>
                                            )}
                                        </td>
                                        <td className="px-4 py-3">
                                            <div className="flex items-center justify-end gap-3">
                                                <Button
                                                    variant="ghost"
                                                    onClick={() => openEdit(user)}
                                                    disabled={!canManageUsers}
                                                    aria-label={`Edit ${user.name}`}
                                                    className="!px-3 !py-1.5 !text-xs"
                                                >
                                                    <Pencil size={13} aria-hidden />
                                                    Edit
                                                </Button>
                                                {isSelf ? (
                                                    <span className="text-xs text-noc-muted">You</span>
                                                ) : (
                                                    <>
                                                        <ToggleSwitch
                                                            tone="accent"
                                                            checked={user.is_active}
                                                            disabled={busyId === user.id || !canManageUsers}
                                                            onChange={(next) => setActive(user, next)}
                                                            label={user.is_active ? `Disable ${user.name}` : `Enable ${user.name}`}
                                                        />
                                                        <Button
                                                            variant="ghost"
                                                            loading={busyId === user.id}
                                                            disabled={!canManageUsers}
                                                            onClick={() => openConfirm(user, 'disable')}
                                                            className="!border-status-down/30 !px-3 !py-1.5 !text-xs !text-status-down hover:!bg-status-down/10"
                                                        >
                                                            Disable
                                                        </Button>
                                                    </>
                                                )}
                                            </div>
                                        </td>
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                    </div>
                </>
            )}

            <UserFormDrawer
                open={drawerOpen}
                onClose={() => setDrawerOpen(false)}
                onSaved={reload}
                user={editing}
            />

            <ConfirmModal
                tone="danger"
                open={confirmUser !== null}
                title={`Disable user "${confirmUser?.name ?? ''}"?`}
                message={
                    confirmAction === 'toggle'
                        ? 'Checks stop and the row leaves the active list.'
                        : 'They will be signed out and cannot sign in again.'
                }
                confirmLabel="Disable"
                pending={busyId === confirmUser?.id}
                onConfirm={() => {
                    if (!confirmUser) return;
                    if (confirmAction === 'toggle') {
                        void runSetActive(confirmUser, false);
                    } else {
                        void runDisable(confirmUser);
                    }
                }}
                onClose={() => setConfirmUser(null)}
            />
        </AppLayout>
    );
}
