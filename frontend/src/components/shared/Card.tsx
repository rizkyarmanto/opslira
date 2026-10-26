import type { ReactNode } from 'react';

/**
 * Solid card surface. Glass is reserved for the top bar only (DESIGN.md, R-10).
 * Elevation is a border plus a hairline ring, never a drop shadow (R-12).
 */
export default function Card({
    children,
    className = '',
    as: Tag = 'div',
}: {
    children: ReactNode;
    className?: string;
    as?: 'div' | 'section' | 'article' | 'li';
}) {
    return (
        <Tag className={`rounded-md border border-noc-border bg-noc-surface ${className}`}>
            {children}
        </Tag>
    );
}
