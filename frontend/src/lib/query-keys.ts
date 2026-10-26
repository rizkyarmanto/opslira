/**
 * TanStack Query key factory. Keeps cache keys consistent across dashboards.
 */
export const queryKeys = {
    session: ['session'] as const,
    dashboard: {
        noc: ['dashboard', 'noc'] as const,
        states: ['dashboard', 'states'] as const,
        alerts: (filter: string) => ['dashboard', 'alerts', filter] as const,
        network: ['dashboard', 'network'] as const,
        appsDomainsDb: (environment: string) => ['dashboard', 'apps-domains-db', environment] as const,
        mailS3Beszel: ['dashboard', 'mail-s3-beszel'] as const,
    },
    targets: {
        all: ['targets'] as const,
        list: (filters: Record<string, unknown>) => ['targets', 'list', filters] as const,
        detail: (id: string) => ['targets', 'detail', id] as const,
        search: (q: string) => ['targets', 'search', q] as const,
    },
    reports: {
        all: ['reports'] as const,
        schedules: ['reports', 'schedules'] as const,
        emailLogs: ['reports', 'email-logs'] as const,
    },
    users: ['users'] as const,
};
