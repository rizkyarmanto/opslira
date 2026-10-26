import { useQuery } from '@tanstack/react-query';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import EmailDeliveryLogs from '@/pages/Reports/EmailDeliveryLogs';
import type { EmailDeliveryLog } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface EmailLogsResponse {
    logs: EmailDeliveryLog[];
    reports: Array<{
        id: string;
        period_type: string;
        period_start: string;
        generation_status: string;
        pdf_file_path: string | null;
        excel_file_path: string | null;
    }>;
}

export default function EmailLogsRoute() {
    const query = useQuery({
        queryKey: queryKeys.reports.emailLogs,
        queryFn: async () => {
            const { data } = await api.get<EmailLogsResponse>('/reports/email-logs');
            return data;
        },
    });

    if (query.isPending) {
        return (
            <AppLayout title="Report & Email History">
                <LoadingState label="Loading report and email history" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Report & Email History">
                <ErrorState
                    message={apiErrorMessage(query.error, 'Could not load report and email history.')}
                    onRetry={() => void query.refetch()}
                />
            </AppLayout>
        );
    }

    return <EmailDeliveryLogs logs={query.data.logs} reports={query.data.reports} />;
}
