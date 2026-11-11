"use client";

import { Suspense, type ReactNode } from "react";
import {
  Building2,
  ClipboardCheck,
  Flag,
  FolderKanban,
  History,
  Inbox,
  LayoutGrid,
  Receipt,
  UserPlus,
  Users,
} from "lucide-react";
import { initialsOf, SignOutButton } from "@/components/shells/account-menu";
import { WorkspaceShell, type WorkspaceNavItem } from "@/components/shells/workspace-shell";
import { brand } from "@/lib/brand";
import { RequireSession, SessionProvider, useUser } from "@/lib/session";

/** Whether the user holds any of the keys. The UI hides what it cannot use; the API decides. */
export function useCan() {
  const user = useUser();
  return (...keys: string[]) =>
    keys.some((k) => user.permissions.includes(k) || user.permissions.includes("system.admin"));
}

// Pages that exist so far; the rest of the sitemap joins in later stages.
const items: Array<WorkspaceNavItem & { perms?: string[] }> = [
  { href: "/w", label: "Overview", icon: LayoutGrid, exact: true },
  {
    href: "/w/projects",
    label: "Projects",
    icon: FolderKanban,
    perms: ["project.manage", "item.import", "gold.manage", "org.project.read"],
  },
  {
    href: "/w/requests",
    label: "Requests",
    icon: Inbox,
    perms: ["project.manage", "org.project.request", "org.project.read"],
  },
  { href: "/w/review", label: "Review", icon: ClipboardCheck, perms: ["review.read"] },
  {
    href: "/w/contributors",
    label: "Contributors",
    icon: Users,
    perms: ["contributor.approve", "contributor.pause"],
  },
  { href: "/w/reports", label: "Reports", icon: Flag, perms: ["project.manage"] },
  { href: "/w/organisations", label: "Organisations", icon: Building2, perms: ["org.manage"] },
  { href: "/w/invoices", label: "Invoices", icon: Receipt, perms: ["invoice.manage", "org.invoice.read"] },
  { href: "/w/team", label: "Team", icon: UserPlus, perms: ["org.user.invite"] },
  { href: "/w/activity", label: "Activity", icon: History, perms: ["org.project.read", "audit.read"] },
];

function Shell({ children }: { children: ReactNode }) {
  const user = useUser();
  const can = useCan();
  const nav = items.filter((i) => !i.perms || can(...i.perms));
  return (
    <WorkspaceShell
      nav={nav}
      user={{
        name: user.display_name,
        initials: initialsOf(user.display_name),
        role: user.kind === "staff" ? `${brand.name} staff` : "Client",
      }}
      orgLabel={user.kind === "staff" ? "ELS operations" : "Client workspace"}
      account={<SignOutButton />}
    >
      {children}
    </WorkspaceShell>
  );
}

// Hooks that read the URL (usePathname) need a Suspense boundary under Cache Components.
const blank = <div className="min-h-dvh bg-surface" aria-busy="true" />;

export function Workspace({ children }: { children: ReactNode }) {
  return (
    <SessionProvider>
      <Suspense fallback={blank}>
        <RequireSession kinds={["staff", "client"]} fallback={blank}>
          <Shell>{children}</Shell>
        </RequireSession>
      </Suspense>
    </SessionProvider>
  );
}

export function PageTitle({ title, children }: { title: ReactNode; children?: ReactNode }) {
  return (
    <header className="mb-6 grid gap-1.5">
      <h1 className="font-display text-2xl font-semibold tracking-[-0.01em]">{title}</h1>
      {children && <div className="text-sm text-fg-muted">{children}</div>}
    </header>
  );
}
