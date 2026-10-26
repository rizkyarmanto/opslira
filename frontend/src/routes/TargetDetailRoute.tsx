import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import TargetDetail from '@/pages/Targets/TargetDetail';
import type { Incident, MonitoringTarget, TargetState } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface DetailResponse {
    target: MonitoringTarget & { state: TargetState; has_credentials: boolean };
    logs: Array<{
        id: number;
        status: string;
        response_time_ms: number | null;
        checked_at: string;
        error_message: string | null;
    }>;
    incidents: Incident[];
}

export default function TargetDetailRoute() {
    const { id = '' } = useParams();

    const query = useQuery({
        queryKey: queryKeys.targets.detail(id),
        queryFn: async () => {
            const { data } = await api.get<DetailResponse>(`/targets/${id}`);
            return data;
        },
        enabled: id !== '',
    });

    if (query.isPending) {
        return (
            <AppLayout title="Target">
                <LoadingState label="Loading target" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Target">
                <ErrorState message={apiErrorMessage(query.error, 'Could not load this target.')} onRetry={() => void query.refetch()} />
            </AppLayout>
        );
    }

    return <TargetDetail target={query.data.target} logs={query.data.logs} incidents={query.data.incidents} />;
}
