import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import AlertsIndex from '@/pages/Dashboard/AlertsIndex';
import type { Incident } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

type AlertFilter = 'open' | 'resolved' | 'all';

interface AlertsResponse {
    incidents: Incident[];
    filter: AlertFilter;
}

const VALID: AlertFilter[] = ['open', 'resolved', 'all'];

export default function AlertsRoute() {
    const [searchParams] = useSearchParams();
    const raw = searchParams.get('filter');
    const filter: AlertFilter = raw === 'resolved' || raw === 'all' ? raw : 'open';

    const query = useQuery({
        queryKey: queryKeys.dashboard.alerts(filter),
        queryFn: async () => {
            const { data } = await api.get<AlertsResponse>('/dashboard/alerts', { params: { filter } });
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
            <AppLayout title="Alerts">
                <LoadingState label="Loading alerts" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Alerts">
                <ErrorState message={apiErrorMessage(query.error, 'Could not load alerts.')} onRetry={() => void query.refetch()} />
            </AppLayout>
        );
    }

    void VALID;

    return (
        <AlertsIndex
            incidents={query.data.incidents}
            filter={query.data.filter}
            refreshing={query.isFetching && !query.isPending}
            lastUpdated={lastUpdated}
        />
    );
}
