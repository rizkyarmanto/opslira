import { useQuery } from '@tanstack/react-query';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import ScheduleManager from '@/pages/Reports/ScheduleManager';
import type { ReportSchedule } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface SchedulesResponse {
    schedules: ReportSchedule[];
}

export default function SchedulesRoute() {
    const query = useQuery({
        queryKey: queryKeys.reports.schedules,
        queryFn: async () => {
            const { data } = await api.get<SchedulesResponse>('/reports/schedules');
            return data;
        },
    });

    if (query.isPending) {
        return (
            <AppLayout title="Schedules & Reports">
                <LoadingState label="Loading report schedules" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Schedules & Reports">
                <ErrorState
                    message={apiErrorMessage(query.error, 'Could not load report schedules.')}
                    onRetry={() => void query.refetch()}
                />
            </AppLayout>
        );
    }

    return <ScheduleManager schedules={query.data.schedules} />;
}
