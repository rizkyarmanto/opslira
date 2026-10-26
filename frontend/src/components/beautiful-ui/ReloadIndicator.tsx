import LatticeLoader from '@/components/beautiful-ui/LatticeLoader';

/**
 * One token-correct call site for the lattice loader while a request is in
 * flight. Callers render it only during a pending request, so it is a bounded
 * state change rather than a permanent loop (DESIGN.md motion rule).
 */
export default function ReloadIndicator({ label = 'Loading' }: { label?: string }) {
    return (
        <span className="inline-flex items-center gap-2 text-xs text-noc-muted">
            <LatticeLoader
                status="working"
                pattern="orbit"
                grid={3}
                shape="round"
                cellSize={4}
                gap={1}
                fontSize={11}
                step={90}
                showTimer={false}
                color="#a1a1aa"
                doneColor="#34d399"
                label={label}
            />
        </span>
    );
}
