/**
 * TanStack Query key factory. Keeps cache keys consistent across dashboards.
 */
export const queryKeys = {
    dashboard: {
        noc: ['dashboard', 'noc'] as const,
        network: ['dashboard', 'network'] as const,
        appsDomainsDb: ['dashboard', 'apps-domains-db'] as const,
        mailS3Beszel: ['dashboard', 'mail-s3-beszel'] as const,
    },
    targets: {
        all: ['targets'] as const,
        list: (filters: Record<string, unknown>) => ['targets', 'list', filters] as const,
        detail: (id: string) => ['targets', 'detail', id] as const,
    },
    reports: {
        schedules: ['reports', 'schedules'] as const,
        emailLogs: ['reports', 'email-logs'] as const,
    },
};
