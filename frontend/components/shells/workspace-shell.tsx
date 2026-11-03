"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";
import {
  Activity,
  Building2,
  ClipboardCheck,
  FolderKanban,
  Inbox,
  LayoutGrid,
  Menu,
  Settings,
  Users,
  Wallet,
  type LucideIcon,
} from "lucide-react";
import { Logo } from "@/components/brand/logo";
import { Badge } from "@/components/ui/display";
import { IconButton } from "@/components/ui/button";
import { Drawer } from "@/components/ui/dialog";
import { brand } from "@/lib/brand";
import { cn } from "@/lib/cn";
import { NavLink } from "./nav-link";

export type WorkspaceNavItem = {
  href: string;
  label: string;
  icon: LucideIcon;
  count?: number;
  exact?: boolean;
};

/** Full Ops navigation. Real pages filter this by the caller's permissions (Stage 4). */
export const opsNav: WorkspaceNavItem[] = [
  { href: "/w", label: "Overview", icon: LayoutGrid, exact: true },
  { href: "/w/projects", label: "Projects", icon: FolderKanban },
  { href: "/w/review", label: "Review", icon: ClipboardCheck },
  { href: "/w/contributors", label: "Contributors", icon: Users },
  { href: "/w/organisations", label: "Organisations", icon: Building2 },
  { href: "/w/payouts", label: "Payouts", icon: Wallet },
  { href: "/w/enquiries", label: "Enquiries", icon: Inbox },
  { href: "/w/activity", label: "Activity", icon: Activity },
  { href: "/w/settings", label: "Settings", icon: Settings },
];

function NavList({ items, onNavigate }: { items: WorkspaceNavItem[]; onNavigate?: () => void }) {
  return (
    <ul className="grid gap-0.5">
      {items.map((n) => (
        <li key={n.href}>
          <NavLink
            href={n.href}
            exact={n.exact}
            onClick={onNavigate}
            className={(active) =>
              cn(
                "flex min-h-10 items-center gap-3 rounded-control px-3 text-sm transition-colors duration-(--dur-quick)",
                active
                  ? "bg-surface-raised text-fg"
                  : "text-fg-muted hover:bg-surface-raised/60 hover:text-fg",
              )
            }
          >
            <n.icon aria-hidden="true" className="size-4" />
            <span className="flex-1">{n.label}</span>
            {n.count ? <Badge>{n.count}</Badge> : null}
          </NavLink>
        </li>
      ))}
    </ul>
  );
}

/**
 * Ops and client workspace: charcoal sidebar, paper content, dense and
 * data-first. Desktop-first; below 1024 px the sidebar becomes a drawer.
 */
export function WorkspaceShell({
  nav = opsNav,
  user,
  orgLabel,
  children,
}: {
  nav?: WorkspaceNavItem[];
  user: { name: string; initials: string; role: string };
  orgLabel: string;
  children: ReactNode;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const userBlock = (
    <div className="flex items-center gap-3 border-t border-border px-3 pt-4">
      <span className="grid size-8 place-items-center rounded-full bg-surface-raised font-mono text-xs font-semibold">
        {user.initials}
      </span>
      <span className="grid min-w-0 text-sm leading-tight">
        <span className="truncate text-fg">{user.name}</span>
        <span className="truncate text-xs text-fg-muted">{user.role}</span>
      </span>
    </div>
  );

  return (
    <div className="min-h-dvh bg-surface lg:grid lg:grid-cols-[15rem_1fr]">
      <a
        href="#main"
        className="sr-only z-50 bg-lime px-4 py-2 font-medium text-charcoal-950 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        Skip to content
      </a>

      <aside
        data-theme="dark"
        className="sticky top-0 hidden h-dvh flex-col gap-6 bg-surface px-3 py-5 lg:flex"
      >
        <Link href="/w" aria-label={`${brand.name} workspace`} className="px-3 text-fg">
          <Logo title="" className="h-6" />
        </Link>
        <p className="px-3 font-mono text-[0.6875rem] tracking-wide text-fg-muted uppercase">{orgLabel}</p>
        <nav aria-label="Workspace" className="flex-1 overflow-y-auto">
          <NavList items={nav} />
        </nav>
        {userBlock}
      </aside>

      <header
        data-theme="dark"
        className="sticky top-0 z-30 flex h-14 items-center gap-3 bg-surface px-2 lg:hidden"
      >
        <IconButton
          label="Open navigation"
          icon={<Menu />}
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen(true)}
        />
        <Link href="/w" aria-label={`${brand.name} workspace`} className="text-fg">
          <Logo title="" className="h-5" />
        </Link>
        <span className="ml-auto truncate pr-2 font-mono text-[0.6875rem] text-fg-muted uppercase">
          {orgLabel}
        </span>
      </header>

      <Drawer
        open={menuOpen}
        onClose={() => setMenuOpen(false)}
        title="Navigation"
        side="left"
        className="lg:hidden"
      >
        <div data-theme="dark" className="-mx-5 -my-4 flex min-h-full flex-col gap-6 bg-surface px-3 py-4">
          <nav aria-label="Workspace (mobile)" className="flex-1">
            <NavList items={nav} onNavigate={() => setMenuOpen(false)} />
          </nav>
          {userBlock}
        </div>
      </Drawer>

      <main id="main" className="min-w-0 px-4 py-6 sm:px-6 lg:px-10 lg:py-8">
        {children}
      </main>
    </div>
  );
}
