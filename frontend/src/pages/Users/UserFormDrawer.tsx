import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import SlideOverDrawer from '@/components/ui-layouts/SlideOverDrawer';
import Button from '@/components/shared/Button';
import ToggleSwitch from '@/components/shared/ToggleSwitch';
import { api, apiErrorMessage } from '@/lib/api';
import { useSession } from '@/lib/session';
import {
    createUserSchema,
    roleLabels,
    updateUserSchema,
    type UserFormValues,
} from '@/lib/schemas/user';
import type { ManagedUser, UserRole } from '@/types/monitoring';

const roles = Object.keys(roleLabels) as UserRole[];

const emptyValues: UserFormValues = {
    name: '',
    email: '',
    role: 'DEVELOPER',
    is_active: true,
    password: '',
    password_confirmation: '',
};

/**
 * Add or edit a user. Edit mode treats a blank password as "keep the current one",
 * matching the server rule in UpdateUserRequest (R-26, R-35).
 */
export default function UserFormDrawer({
    open,
    onClose,
    onSaved,
    user,
}: {
    open: boolean;
    onClose: () => void;
    onSaved: () => void;
    user: ManagedUser | null;
}) {
    const isEdit = user !== null;
    const { user: currentUser } = useSession();
    const isSelf = isEdit && user?.id === currentUser?.id;

    const [saving, setSaving] = useState(false);
    const [saveError, setSaveError] = useState<string | null>(null);

    const {
        register,
        handleSubmit,
        watch,
        setValue,
        reset,
        formState: { errors },
    } = useForm<UserFormValues>({
        resolver: zodResolver(isEdit ? updateUserSchema : createUserSchema),
        defaultValues: emptyValues,
    });

    // Reload the form whenever the drawer opens or a different row is selected, so
    // the fields always reflect the row that was clicked.
    useEffect(() => {
        if (!open) return;
        setSaveError(null);
        reset(
            user
                ? {
                      name: user.name,
                      email: user.email,
                      role: user.role,
                      is_active: user.is_active,
                      password: '',
                      password_confirmation: '',
                  }
                : emptyValues,
        );
    }, [open, user, reset]);

    const isActive = watch('is_active');

    const onSubmit = async (values: UserFormValues) => {
        setSaving(true);
        setSaveError(null);
        try {
            if (isEdit && user) {
                await api.put(`/users/${user.id}`, values);
            } else {
                await api.post('/users', values);
            }
            reset(emptyValues);
            onSaved();
            onClose();
        } catch (error) {
            setSaveError(apiErrorMessage(error, 'Failed to save the user. Please review the form fields.'));
        } finally {
            setSaving(false);
        }
    };

    return (
        <SlideOverDrawer
            open={open}
            onClose={onClose}
            title={isEdit ? 'Edit user' : 'Add user'}
            description="Set the account details and the role that decides what this person can reach."
            footer={
                <div className="flex items-center justify-end gap-3">
                    <Button variant="ghost" onClick={onClose}>
                        Cancel
                    </Button>
                    <Button loading={saving} onClick={handleSubmit(onSubmit)}>
                        Save
                    </Button>
                </div>
            }
        >
            <form className="space-y-4" onSubmit={handleSubmit(onSubmit)} noValidate>
                <Field label="Name" error={errors.name?.message} htmlFor="user-name">
                    <input id="user-name" {...register('name')} className="input" autoFocus placeholder="Example: Budi Santoso" />
                </Field>

                <Field label="Email" error={errors.email?.message} htmlFor="user-email">
                    <input id="user-email" type="email" {...register('email')} className="input tabular" placeholder="name@opslira.local" />
                </Field>

                <Field label="Role" error={errors.role?.message} htmlFor="user-role">
                    <select id="user-role" {...register('role')} className="input" disabled={isSelf}>
                        {roles.map((role) => (
                            <option key={role} value={role}>
                                {roleLabels[role]}
                            </option>
                        ))}
                    </select>
                </Field>

                <Field
                    label="Password"
                    error={errors.password?.message}
                    htmlFor="user-password"
                    hint={isEdit ? 'Leave blank to keep the current password.' : undefined}
                >
                    <input
                        id="user-password"
                        type="password"
                        autoComplete="new-password"
                        {...register('password')}
                        className="input"
                        placeholder={isEdit ? 'Leave blank to keep' : 'At least 8 characters'}
                    />
                </Field>

                <Field
                    label="Confirm password"
                    error={errors.password_confirmation?.message}
                    htmlFor="user-password-confirm"
                >
                    <input
                        id="user-password-confirm"
                        type="password"
                        autoComplete="new-password"
                        {...register('password_confirmation')}
                        className="input"
                    />
                </Field>

                <div className="flex items-center justify-between gap-3 rounded-sm border border-noc-border px-4 py-3">
                    <span className="field-label">Account is active</span>
                    <ToggleSwitch
                        tone="accent"
                        checked={isActive}
                        disabled={isSelf}
                        onChange={(next) => setValue('is_active', next)}
                        label="Account is active"
                    />
                </div>

                {isSelf && (
                    <p className="text-xs text-noc-muted">
                        You cannot change your own role or disable your own account.
                    </p>
                )}

                {saveError && (
                    <p role="alert" className="rounded-sm border border-status-down/40 bg-status-down/10 px-3 py-2 text-xs text-status-down">
                        {saveError}
                    </p>
                )}
            </form>
        </SlideOverDrawer>
    );
}

function Field({
    label,
    error,
    hint,
    htmlFor,
    className = '',
    children,
}: {
    label: string;
    error?: string;
    hint?: string;
    htmlFor: string;
    className?: string;
    children: React.ReactNode;
}) {
    return (
        <div className={className}>
            <label htmlFor={htmlFor} className="field-label mb-1.5 block">
                {label}
            </label>
            {children}
            {hint && <p className="mt-1 text-[11px] text-noc-muted">{hint}</p>}
            {error && (
                <p role="alert" className="mt-1 text-[11px] text-status-down">
                    {error}
                </p>
            )}
        </div>
    );
}
