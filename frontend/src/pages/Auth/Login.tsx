import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
import { api, apiErrorMessage, apiFieldErrors } from '@/lib/api';
import { Head } from '@/lib/head';
import { queryKeys } from '@/lib/query-keys';

interface LoginValues {
    email: string;
    password: string;
}

export default function Login() {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const [formError, setFormError] = useState<string | null>(null);

    const {
        register,
        handleSubmit,
        setError,
        formState: { errors, isSubmitting },
    } = useForm<LoginValues>({
        defaultValues: { email: '', password: '' },
    });

    const submit = async (values: LoginValues) => {
        setFormError(null);
        try {
            await api.post('/auth/login', values);
            // Load the new session before navigating so the shell opens signed in.
            await queryClient.invalidateQueries({ queryKey: queryKeys.session });
            navigate('/dashboard/noc', { replace: true });
        } catch (error) {
            const fieldErrors = apiFieldErrors(error);
            let mapped = false;
            for (const field of ['email', 'password'] as const) {
                if (fieldErrors[field]) {
                    setError(field, { type: 'server', message: fieldErrors[field] });
                    mapped = true;
                }
            }
            if (!mapped) {
                setFormError(apiErrorMessage(error, 'Could not sign in. Please try again.'));
            }
        }
    };

    return (
        <div className="flex min-h-screen items-center justify-center bg-noc-bg px-4">
            <Head title="Sign in" />

            <div className="w-full max-w-sm rounded-md border border-noc-border bg-noc-surface p-8">
                <div className="mb-6 flex items-center gap-3">
                    <img src="/images/opslira.jpg" alt="" className="h-10 w-10 rounded-sm object-cover" />
                    <div>
                        <h1 className="font-display text-lg font-bold tracking-tight">OPSLIRA</h1>
                        <p className="text-[11px] text-noc-muted">Internal infrastructure monitoring</p>
                    </div>
                </div>

                <form onSubmit={handleSubmit(submit)} className="space-y-4" noValidate>
                    <div>
                        <label htmlFor="email" className="field-label mb-1.5 block">
                            Email
                        </label>
                        <input
                            id="email"
                            type="email"
                            autoComplete="username"
                            autoFocus
                            {...register('email')}
                            aria-invalid={errors.email ? true : undefined}
                            aria-describedby={errors.email ? 'email-error' : undefined}
                            className="input tabular"
                        />
                        {errors.email && (
                            <p id="email-error" role="alert" className="mt-1 text-[11px] text-status-down">
                                {errors.email.message}
                            </p>
                        )}
                    </div>

                    <div>
                        <label htmlFor="password" className="field-label mb-1.5 block">
                            Password
                        </label>
                        <input
                            id="password"
                            type="password"
                            autoComplete="current-password"
                            {...register('password')}
                            aria-invalid={errors.password ? true : undefined}
                            aria-describedby={errors.password ? 'password-error' : undefined}
                            className="input"
                        />
                        {errors.password && (
                            <p id="password-error" role="alert" className="mt-1 text-[11px] text-status-down">
                                {errors.password.message}
                            </p>
                        )}
                    </div>

                    {formError && (
                        <p role="alert" className="text-[11px] text-status-down">
                            {formError}
                        </p>
                    )}

                    <button type="submit" disabled={isSubmitting} className="btn-primary w-full" aria-busy={isSubmitting || undefined}>
                        {isSubmitting && <Loader2 size={16} className="animate-spin" aria-hidden />}
                        Sign in
                    </button>
                </form>
            </div>
        </div>
    );
}
