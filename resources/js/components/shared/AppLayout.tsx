import type { ReactNode } from 'react';
import { Link, usePage } from '@inertiajs/react';
import {
    FileText,
    Gauge,
    Mail,
    Network,
    Server,
    Settings,
    Target,
    Users,
} from 'lucide-react';
import CommandPalette from '@/components/beautiful-ui/CommandPalette';
import FlashMessages from '@/components/shared/FlashMessages';
import ErrorBoundary from '@/components/shared/ErrorBoundary';
import type { SharedPageProps } from '@/types/inertia';

interface NavItem {
    label: string;
    href: string;
    icon: ReactNode;
    match: string;
}

/**
 * Navigation reflects the pages that exist, nothing more (R-24).
 * Icons name what the section holds: gauge for the overview, network for devices,
 * server for apps and databases, mail for the mail and storage group.
 */
const navItems: NavItem[] = [
    { label: 'NOC Overview', href: '/dashboard/noc', icon: <Gauge size={16} aria-hidden />, match: '/dashboard/noc' },
    { label: 'Network', href: '/dashboard/network', icon: <Network size={16} aria-hidden />, match: '/dashboard/network' },
    { label: 'App, Domain & DB', href: '/dashboard/apps-domains-db', icon: <Server size={16} aria-hidden />, match: '/dashboard/apps-domains-db' },
    { label: 'Mail, S3 & Beszel', href: '/dashboard/mail-s3-beszel', icon: <Mail size={16} aria-hidden />, match: '/dashboard/mail-s3-beszel' },
    { label: 'Targets', href: '/targets', icon: <Target size={16} aria-hidden />, match: '/targets' },
    { label: 'Schedules & Reports', href: '/reports/schedules', icon: <FileText size={16} aria-hidden />, match: '/reports' },
];

const usersNavItem: NavItem = {
    label: 'Users',
    href: '/users',
    icon: <Users size={16} aria-hidden />,
    match: '/users',
};

export default function AppLayout({ children, title }: { children: ReactNode; title?: string }) {
    const { url, props } = usePage<SharedPageProps>();
    const user = props.auth?.user;

    // User administration is shown only to a Super Admin (see EnsureSuperAdmin).
    const items = props.can?.manageUsers ? [...navItems, usersNavItem] : navItems;

    return (
        <div className="flex min-h-screen flex-col bg-noc-bg text-noc-text">
            <a
                href="#main"
                className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-sm focus:bg-noc-accent focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-zinc-950"
            >
                Skip to main content
            </a>

            {/* Top bar: the one glass surface in the app (DESIGN.md, R-10) */}
            <header className="sticky top-0 z-30 border-b border-noc-border bg-noc-bg/85 backdrop-blur-md">
                <div className="mx-auto flex h-16 max-w-[1600px] items-center gap-4 px-4 md:px-6">
                    <Link href="/dashboard/noc" className="flex items-center gap-2">
                        <img src="/images/opslira.jpg" alt="" className="h-8 w-8 rounded-xs object-cover" />
                        <span className="font-display text-base font-bold tracking-tight">OPSLIRA</span>
                    </Link>

                    <div className="hidden flex-1 justify-center md:flex">
                        <CommandPalette />
                    </div>

                    <div className="ml-auto flex items-center gap-2">
                        {user ? (
                            <div className="text-right">
                                <p className="text-xs font-medium text-noc-text">{user.name}</p>
                                <p className="text-[10px] uppercase tracking-wide text-noc-muted">
                                    {user.role.replace(/_/g, ' ')}
                                </p>
                            </div>
                        ) : null}
                        <Link
                            href="/settings"
                            aria-label="Settings"
                            className="tap-target inline-flex items-center justify-center rounded-sm text-noc-muted hover:bg-white/5 hover:text-noc-text"
                        >
                            <Settings size={18} aria-hidden />
                        </Link>
                    </div>
                </div>

                <nav
                    aria-label="Main navigation"
                    className="mx-auto flex max-w-[1600px] items-center gap-1 overflow-x-auto px-2 md:px-4"
                >
                    {items.map((item) => {
                        const active = url.startsWith(item.match);
                        return (
                            <Link
                                key={item.href}
                                href={item.href}
                                aria-current={active ? 'page' : undefined}
                                className={`tap-target flex shrink-0 items-center gap-2 border-b-2 px-3 py-2 text-sm transition-colors ${
                                    active
                                        ? 'border-noc-accent text-noc-text'
                                        : 'border-transparent text-noc-muted hover:text-noc-text'
                                }`}
                            >
                                {item.icon}
                                {item.label}
                            </Link>
                        );
                    })}
                </nav>
            </header>

            <main id="main" className="mx-auto w-full max-w-[1600px] flex-1 px-4 py-6 md:px-6">
                <FlashMessages />
                {title && (
                    <div className="mb-6">
                        <h1 className="font-display text-display-sm">{title}</h1>
                    </div>
                )}
                <ErrorBoundary title={title}>{children}</ErrorBoundary>
            </main>

            <footer className="border-t border-noc-border px-4 py-4 md:px-6">
                <p className="mx-auto max-w-[1600px] text-xs text-noc-muted">
                    Opslira, internal office infrastructure monitoring.
                </p>
            </footer>
        </div>
    );
}
