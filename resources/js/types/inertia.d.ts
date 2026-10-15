import type { PageProps } from '@inertiajs/core';

export interface AuthUser {
    id: string;
    name: string;
    email: string;
    role: string;
}

export interface FlashMessages {
    success?: string | null;
    error?: string | null;
    message?: string | null;
}

export interface SharedPageProps extends PageProps {
    auth: {
        user: AuthUser | null;
    };
    flash: FlashMessages;
    can: {
        manageUsers: boolean;
    };
}
