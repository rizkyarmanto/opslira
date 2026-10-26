import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import TargetIndex from '@/pages/Targets/TargetIndex';
import type { MonitoringTarget } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface TargetsResponse {
    targets: MonitoringTarget[];
    filters: Record<string, string>;
}

export default function TargetsRoute() {
    const [searchParams] = useSearchParams();
    const filters = {
        dashboard_group: searchParams.get('dashboard_group') ?? '',
        target_type: searchParams.get('target_type') ?? '',
        environment: searchParams.get('environment') ?? '',
        status: searchParams.get('status') ?? '',
        state: searchParams.get('state') ?? '',
        maintenance: searchParams.get('maintenance') ?? '',
    };

    const query = useQuery({
        queryKey: queryKeys.targets.list(filters),
        queryFn: async () => {
            const params: Record<string, string> = {};
            for (const [key, value] of Object.entries(filters)) {
                if (value) params[key] = value;
            }
            const { data } = await api.get<TargetsResponse>('/targets', { params });
            return data;
        },
    });

    if (query.isPending) {
        return (
            <AppLayout title="Targets">
                <LoadingState label="Loading targets" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Targets">
                <ErrorState message={apiErrorMessage(query.error, 'Could not load targets.')} onRetry={() => void query.refetch()} />
            </AppLayout>
        );
    }

    return <TargetIndex targets={query.data.targets} filters={query.data.filters} />;
}
