import { z } from 'zod';
import type { UserRole } from '@/types/monitoring';

/**
 * User form schemas. Create requires a password; edit treats a blank password as
 * "keep the current one", which mirrors what UpdateUserRequest accepts on the server.
 */

const baseFields = {
    name: z.string().min(1, 'Name is required').max(120),
    email: z.string().min(1, 'Email is required').email('Enter a valid email address').max(150),
    role: z.enum(['SUPER_ADMIN', 'SYSADMIN', 'DEVELOPER', 'MANAGER']),
    is_active: z.boolean().default(true),
};

export const createUserSchema = z
    .object({
        ...baseFields,
        password: z.string().min(8, 'Use at least 8 characters'),
        password_confirmation: z.string(),
    })
    .refine((v) => v.password === v.password_confirmation, {
        message: 'The two passwords do not match',
        path: ['password_confirmation'],
    });

export const updateUserSchema = z
    .object({
        ...baseFields,
        password: z.string().optional(),
        password_confirmation: z.string().optional(),
    })
    .refine((v) => !v.password || v.password.length >= 8, {
        message: 'Use at least 8 characters',
        path: ['password'],
    })
    .refine((v) => (v.password ?? '') === (v.password_confirmation ?? ''), {
        message: 'The two passwords do not match',
        path: ['password_confirmation'],
    });

export type UserFormValues = z.infer<typeof createUserSchema>;

export const roleLabels: Record<UserRole, string> = {
    SUPER_ADMIN: 'Super Admin',
    SYSADMIN: 'Network & SysAdmin',
    DEVELOPER: 'Developer & DBA',
    MANAGER: 'IT Manager',
};
