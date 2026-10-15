import { Head, useForm } from '@inertiajs/react';
import { Loader2 } from 'lucide-react';

export default function Login() {
    const { data, setData, post, processing, errors } = useForm({
        email: '',
        password: '',
    });

    const submit = (e: React.FormEvent) => {
        e.preventDefault();
        post('/login');
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

                <form onSubmit={submit} className="space-y-4" noValidate>
                    <div>
                        <label htmlFor="email" className="field-label mb-1.5 block">
                            Email
                        </label>
                        <input
                            id="email"
                            type="email"
                            name="email"
                            autoComplete="username"
                            autoFocus
                            value={data.email}
                            onChange={(e) => setData('email', e.target.value)}
                            aria-invalid={errors.email ? true : undefined}
                            aria-describedby={errors.email ? 'email-error' : undefined}
                            className="input tabular"
                        />
                        {errors.email && (
                            <p id="email-error" role="alert" className="mt-1 text-[11px] text-status-down">
                                {errors.email}
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
                            name="password"
                            autoComplete="current-password"
                            value={data.password}
                            onChange={(e) => setData('password', e.target.value)}
                            aria-invalid={errors.password ? true : undefined}
                            aria-describedby={errors.password ? 'password-error' : undefined}
                            className="input"
                        />
                        {errors.password && (
                            <p id="password-error" role="alert" className="mt-1 text-[11px] text-status-down">
                                {errors.password}
                            </p>
                        )}
                    </div>

                    <button type="submit" disabled={processing} className="btn-primary w-full" aria-busy={processing || undefined}>
                        {processing && <Loader2 size={16} className="animate-spin" aria-hidden />}
                        Sign in
                    </button>
                </form>
            </div>
        </div>
    );
}
