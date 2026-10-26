import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import AppDbDomainDashboard from '@/pages/Dashboard/AppDbDomainDashboard';
import type { MonitoringTarget } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface AppsResponse {
    targets: MonitoringTarget[];
    environment: string;
    counts: Record<string, number>;
}

export default function AppsRoute() {
    const [searchParams] = useSearchParams();
    const environment = searchParams.get('environment') ?? 'ALL';

    const query = useQuery({
        queryKey: queryKeys.dashboard.appsDomainsDb(environment),
        queryFn: async () => {
            const { data } = await api.get<AppsResponse>('/dashboard/apps-domains-db', {
                params: { environment },
            });
            return data;
        },
        refetchInterval: 15_000,
    });

    const lastUpdated = useMemo(
        () => (query.dataUpdatedAt > 0 ? new Date(query.dataUpdatedAt) : null),
        [query.dataUpdatedAt],
    );

    if (query.isPending) {
        return (
            <AppLayout title="Apps, Domains & Databases">
                <LoadingState label="Loading apps, domains and databases" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Apps, Domains & Databases">
                <ErrorState
                    message={apiErrorMessage(query.error, 'Could not load apps, domains and databases.')}
                    onRetry={() => void query.refetch()}
                />
            </AppLayout>
        );
    }

    return (
        <AppDbDomainDashboard
            targets={query.data.targets}
            environment={query.data.environment}
            counts={query.data.counts}
            lastUpdated={lastUpdated}
        />
    );
}
