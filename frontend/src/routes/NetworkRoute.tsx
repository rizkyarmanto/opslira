import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import NetworkDashboard from '@/pages/Dashboard/NetworkDashboard';
import type { MonitoringTarget } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface NetworkResponse {
    targets: MonitoringTarget[];
    // Go API answers envelope keys in snake_case.
    internet_targets?: MonitoringTarget[];
    traffic_summary?: Record<string, { rx_mbps: number; tx_mbps: number }>;
}

export default function NetworkRoute() {
    const query = useQuery({
        queryKey: queryKeys.dashboard.network,
        queryFn: async () => {
            const { data } = await api.get<NetworkResponse>('/dashboard/network');
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
            <AppLayout title="Network">
                <LoadingState label="Loading network status" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Network">
                <ErrorState message={apiErrorMessage(query.error, 'Could not load network status.')} onRetry={() => void query.refetch()} />
            </AppLayout>
        );
    }

    return (
        <NetworkDashboard
            targets={query.data.targets ?? []}
            internetTargets={query.data.internet_targets ?? []}
            trafficSummary={query.data.traffic_summary ?? {}}
            refreshing={query.isFetching && !query.isPending}
            lastUpdated={lastUpdated}
        />
    );
}
