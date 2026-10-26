import { Component, type ErrorInfo, type ReactNode } from 'react';
import { AlertTriangle, RotateCcw } from 'lucide-react';

/**
 * The error panel itself, shared by the render-time ErrorBoundary and the
 * screens whose data request failed: same wording, same single action (R-27).
 */
export function ErrorPanel({ title, detail }: { title?: string; detail: string }) {
    return (
        <div role="alert" className="rounded-md border border-status-down/40 bg-status-down/5 px-6 py-8 text-center">
            <AlertTriangle size={20} className="mx-auto text-status-down" aria-hidden />
            <h2 className="mt-3 text-sm font-semibold text-noc-text">
                Failed to load {title ?? 'this page'}
            </h2>
            <p className="mx-auto mt-1 max-w-lg text-xs text-noc-muted">
                Something went wrong while displaying the data. Try reloading the page. If it still fails, check the
                application log for details.
            </p>
            <p className="tabular mx-auto mt-3 max-w-lg overflow-x-auto rounded-sm border border-noc-border bg-noc-bg px-3 py-2 text-left text-[11px] text-noc-muted">
                {detail}
            </p>
            <button onClick={() => window.location.reload()} className="btn-primary mt-4">
                <RotateCcw size={15} aria-hidden />
                Reload page
            </button>
        </div>
    );
}

/**
 * Error state for a page whose render failed.
 *
 * Without this a render error leaves a blank screen, which tells the user nothing.
 * This names what failed and offers the one action that can help: reload the page
 * (R-27).
 */
export default class ErrorBoundary extends Component<
    { children: ReactNode; title?: string },
    { error: Error | null }
> {
    state: { error: Error | null } = { error: null };

    static getDerivedStateFromError(error: Error) {
        return { error };
    }

    componentDidCatch(error: Error, info: ErrorInfo) {
        console.error('Page render failed:', error, info.componentStack);
    }

    render() {
        const { error } = this.state;

        if (!error) return this.props.children;

        return <ErrorPanel title={this.props.title} detail={error.message} />;
    }
}
