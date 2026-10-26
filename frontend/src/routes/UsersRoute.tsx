import { useQuery } from '@tanstack/react-query';
import AppLayout from '@/components/shared/AppLayout';
import { api, apiErrorMessage } from '@/lib/api';
import { queryKeys } from '@/lib/query-keys';
import UserIndex from '@/pages/Users/UserIndex';
import type { ManagedUser } from '@/types/monitoring';
import { ErrorState, LoadingState } from '@/routes/QueryState';

interface UsersResponse {
    users: ManagedUser[];
    canManageUsers?: boolean;
    // Older builds answered with the snake key; the route is super_admin only,
    // which is what Laravel's controller always answered.
    can_manage_users?: boolean;
}

export default function UsersRoute() {
    const query = useQuery({
        queryKey: queryKeys.users,
        queryFn: async () => {
            const { data } = await api.get<UsersResponse>('/users');
            return data;
        },
    });

    if (query.isPending) {
        return (
            <AppLayout title="Users">
                <LoadingState label="Loading users" />
            </AppLayout>
        );
    }

    if (query.isError || !query.data) {
        return (
            <AppLayout title="Users">
                <ErrorState message={apiErrorMessage(query.error, 'Could not load users.')} onRetry={() => void query.refetch()} />
            </AppLayout>
        );
    }

    return (
        <UserIndex
            users={query.data.users}
            canManageUsers={query.data.canManageUsers ?? query.data.can_manage_users ?? true}
        />
    );
}
