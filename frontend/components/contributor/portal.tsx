"use client";

import { Suspense, createContext, useContext, type ReactNode } from "react";
import { ClipboardCheck, LayoutDashboard, ListChecks, UserRound } from "lucide-react";
import { AccountMenu, initialsOf } from "@/components/shells/account-menu";
import { ContributorShell, type ContributorNavItem } from "@/components/shells/contributor-shell";
import { Alert, Skeleton } from "@/components/ui/display";
import { Button } from "@/components/ui/button";
import { api, type ContributorProfile } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { RequireSession, SessionProvider, useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { ConsentPrompt } from "./consent-prompt";

// Only pages that exist are linked; Earnings and Training join in Stages 12
// and 14 (sitemap).
const nav: ContributorNavItem[] = [
  { href: "/c", label: "Dashboard", icon: LayoutDashboard, exact: true },
  { href: "/c/work", label: "Tasks", icon: ListChecks },
  { href: "/c/qualify", label: "Qualification", icon: ClipboardCheck },
  { href: "/c/profile", label: "Profile", icon: UserRound },
];

type ProfileContext = {
  profile: ContributorProfile;
  setProfile: (p: ContributorProfile) => void;
  reload: () => void;
};
const Ctx = createContext<ProfileContext | null>(null);

/** The signed-in contributor's profile, loaded once by the portal layout. */
export function useProfile() {
  const v = useContext(Ctx);
  if (!v) throw new Error("useProfile must be used inside the contributor portal");
  return v;
}

function PageSkeleton() {
  return (
    <div className="grid gap-4" aria-busy="true" aria-label="Loading">
      <Skeleton className="h-9 w-64" />
      <Skeleton className="h-5 w-80" />
      <Skeleton className="mt-6 h-48 w-full" />
    </div>
  );
}

function Portal({ children }: { children: ReactNode }) {
  const user = useUser();
  const profile = useLoad(() => api<ContributorProfile>("GET", "/v1/me/contributor"));
  const initials = initialsOf(user.display_name);
  return (
    <ContributorShell
      initials={initials}
      nav={nav}
      account={<AccountMenu initials={initials} name={user.display_name} email={user.email} />}
    >
      {profile.status === "loading" && <PageSkeleton />}
      {profile.status === "error" && (
        <Alert
          tone="danger"
          title={errorMessage(profile.error)}
          action={
            <Button variant="secondary" size="sm" onClick={profile.reload}>
              Try again
            </Button>
          }
        />
      )}
      {profile.status === "ready" && (
        <Ctx.Provider value={{ profile: profile.data, setProfile: profile.set, reload: profile.reload }}>
          {profile.data.consent_required.length > 0 && profile.data.status !== "removed" ? (
            <ConsentPrompt />
          ) : (
            children
          )}
        </Ctx.Provider>
      )}
    </ContributorShell>
  );
}

// Hooks that read the URL (usePathname) need a Suspense boundary under Cache Components.
const blank = <div className="min-h-dvh bg-bg" aria-busy="true" />;

export function ContributorPortal({ children }: { children: ReactNode }) {
  return (
    <SessionProvider>
      <Suspense fallback={blank}>
        <RequireSession kinds={["contributor"]} fallback={blank}>
          <Portal>{children}</Portal>
        </RequireSession>
      </Suspense>
    </SessionProvider>
  );
}
