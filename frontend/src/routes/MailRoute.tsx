import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import MailS3BeszelDashboard from '@/pages/Dashboard/MailS3BeszelDashboard';
import type { MonitoringTarget } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface MailResponse {
    targets: MonitoringTarget[];
}

export default function MailRoute() {
    const query = useQuery({
        queryKey: queryKeys.dashboard.mailS3Beszel,
        queryFn: async () => {
            const { data } = await api.get<MailResponse>('/dashboard/mail-s3-beszel');
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
            <AppLayout title="Mail, S3 & Beszel">
                <LoadingState label="Loading mail, storage and servers" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Mail, S3 & Beszel">
                <ErrorState
                    message={apiErrorMessage(query.error, 'Could not load mail, storage and servers.')}
                    onRetry={() => void query.refetch()}
                />
            </AppLayout>
        );
    }

    return <MailS3BeszelDashboard targets={query.data.targets} lastUpdated={lastUpdated} />;
}
