import Card from '@/components/shared/Card';
import Button from '@/components/shared/Button';
import ReloadIndicator from '@/components/beautiful-ui/ReloadIndicator';

export function LoadingState({ label }: { label: string }) {
    return (
        <Card className="flex items-center justify-center px-4 py-16">
            <ReloadIndicator label={label} />
        </Card>
    );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
    return (
        <Card className="px-4 py-10 text-center">
            <p role="alert" className="mb-1 text-sm font-medium text-noc-text">
                Could not load this screen.
            </p>
            <p className="mx-auto mb-4 max-w-md text-xs text-noc-muted">{message}</p>
            <Button variant="ghost" onClick={onRetry}>
                Try again
            </Button>
        </Card>
    );
}
