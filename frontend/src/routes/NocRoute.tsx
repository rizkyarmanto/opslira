import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import NocOverview from '@/pages/Dashboard/NocOverview';
import type { Incident, MonitoringTarget, NocSummary, TargetStatus } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface NocResponse {
    summary: NocSummary & { maintenance: number };
    targets: MonitoringTarget[];
    incidents: Incident[];
    // Go API answers envelope keys in snake_case.
    global_sla?: number | null;
    sla_window_days?: number | null;
    sla_trend?: Array<{ date: string; uptime: number | null; avg_latency: number | null; incidents: number }> | null;
    target_daily_sla?: Array<{
        target_id: string;
        name: string;
        current_status: TargetStatus;
        uptime: number;
        total_checks: number;
        failed_checks: number;
    }> | null;
    sla_date?: string | null;
}

interface StatesResponse {
    statuses: Record<string, { current_status: TargetStatus; last_checked_at: string | null }>;
}

export default function NocRoute() {
    const noc = useQuery({
        queryKey: queryKeys.dashboard.noc,
        queryFn: async () => {
            const { data } = await api.get<NocResponse>('/dashboard/noc');
            return data;
        },
        refetchInterval: 60_000,
    });

    const states = useQuery({
        queryKey: queryKeys.dashboard.states,
        queryFn: async () => {
            const { data } = await api.get<StatesResponse>('/dashboard/states');
            return data.statuses ?? {};
        },
        refetchInterval: 15_000,
    });

    const lastUpdated = useMemo(() => {
        if (states.dataUpdatedAt > 0) return new Date(states.dataUpdatedAt);
        if (noc.dataUpdatedAt > 0) return new Date(noc.dataUpdatedAt);
        return null;
    }, [states.dataUpdatedAt, noc.dataUpdatedAt]);

    if (noc.isPending) {
        return (
            <AppLayout title="NOC Overview">
                <LoadingState label="Loading NOC overview" />
            </AppLayout>
        );
    }

    if (noc.isError || !noc.data) {
        return (
            <AppLayout title="NOC Overview">
                <ErrorState message={apiErrorMessage(noc.error, 'Could not load the NOC overview.')} onRetry={() => void noc.refetch()} />
            </AppLayout>
        );
    }

    return (
        <NocOverview
            summary={noc.data.summary}
            targets={noc.data.targets}
            incidents={noc.data.incidents}
            globalSla={noc.data.global_sla ?? null}
            slaWindowDays={noc.data.sla_window_days ?? null}
            slaTrend={noc.data.sla_trend ?? null}
            targetDailySla={noc.data.target_daily_sla ?? null}
            slaDate={noc.data.sla_date ?? null}
            statuses={states.data ?? {}}
            refreshing={noc.isFetching && !noc.isPending}
            lastUpdated={lastUpdated}
        />
    );
}
