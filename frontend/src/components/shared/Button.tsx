import type { ButtonHTMLAttributes, ReactNode } from 'react';
import { Loader2 } from 'lucide-react';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';

const variants: Record<Variant, string> = {
    primary: 'btn-primary',
    secondary: 'btn-secondary',
    ghost: 'btn-ghost',
    danger: 'btn border border-status-down/40 text-status-down hover:bg-status-down/10',
};

/**
 * Button with a pending state. While `loading` is true the control is disabled and
 * shows a spinner with a label, so the user can tell work is in progress (R-27).
 */
export default function Button({
    loading = false,
    variant = 'primary',
    children,
    className = '',
    disabled,
    ...rest
}: {
    loading?: boolean;
    variant?: Variant;
    children: ReactNode;
    className?: string;
} & ButtonHTMLAttributes<HTMLButtonElement>) {
    return (
        <button
            {...rest}
            disabled={disabled || loading}
            aria-busy={loading || undefined}
            className={`${variants[variant]} ${className}`}
        >
            {loading && <Loader2 size={15} className="animate-spin" aria-hidden />}
            {children}
        </button>
    );
}
