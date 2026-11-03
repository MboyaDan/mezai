"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { BookOpen, LayoutDashboard, ListChecks, Wallet } from "lucide-react";
import { Logo } from "@/components/brand/logo";
import { brand } from "@/lib/brand";
import { cn } from "@/lib/cn";
import { NavLink } from "./nav-link";

const nav = [
  { href: "/c", label: "Dashboard", icon: LayoutDashboard, exact: true },
  { href: "/c/work", label: "Tasks", icon: ListChecks },
  { href: "/c/earnings", label: "Earnings", icon: Wallet },
  { href: "/c/training", label: "Training", icon: BookOpen },
];

/**
 * Contributor portal: calm ivory canvas, charcoal top bar, larger targets,
 * mobile-first with a bottom tab bar under 768 px.
 */
export function ContributorShell({ initials, children }: { initials: string; children: ReactNode }) {
  return (
    <div className="flex min-h-dvh flex-col bg-bg">
      <a
        href="#main"
        className="sr-only z-50 bg-lime px-4 py-2 font-medium text-charcoal-950 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        Skip to content
      </a>
      <header data-theme="dark" className="sticky top-0 z-30 bg-bg">
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4 md:h-16 md:gap-10 md:px-10">
          <Link
            href="/c"
            aria-label={`${brand.name} contributor dashboard`}
            className="flex items-center gap-4 text-fg"
          >
            <Logo title="" className="h-6" />
            <span className="hidden border-l border-border pl-4 text-sm text-fg-muted sm:inline">
              Contributor
            </span>
          </Link>
          <nav aria-label="Contributor" className="ml-auto hidden gap-1 md:flex">
            {nav.map((n) => (
              <NavLink
                key={n.href}
                href={n.href}
                exact={n.exact}
                className={(active) =>
                  cn(
                    "rounded-control px-3 py-2 text-sm transition-colors duration-(--dur-quick)",
                    active ? "bg-surface-raised text-fg" : "text-fg-muted hover:text-fg",
                  )
                }
              >
                {n.label}
              </NavLink>
            ))}
          </nav>
          <Link
            href="/account"
            className="ml-auto grid size-9 place-items-center rounded-full border border-border bg-surface-raised font-mono text-xs font-semibold text-fg md:ml-0"
          >
            <span className="sr-only">Account </span>
            {initials}
          </Link>
        </div>
      </header>

      <main id="main" className="mx-auto w-full max-w-7xl flex-1 px-4 pt-6 pb-28 md:px-10 md:pt-10 md:pb-16">
        {children}
      </main>

      <nav
        aria-label="Contributor (mobile)"
        data-theme="dark"
        className="fixed inset-x-0 bottom-0 z-30 border-t border-border bg-bg pb-[env(safe-area-inset-bottom)] md:hidden"
      >
        <ul className="grid grid-cols-4">
          {nav.map((n) => (
            <li key={n.href}>
              <NavLink
                href={n.href}
                exact={n.exact}
                className={(active) =>
                  cn(
                    "flex min-h-16 flex-col items-center justify-center gap-1 text-[0.6875rem]",
                    active ? "text-fg" : "text-fg-muted",
                  )
                }
              >
                <n.icon aria-hidden="true" className="size-5" />
                {n.label}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>
    </div>
  );
}
