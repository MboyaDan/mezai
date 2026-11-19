import { Suspense, type ReactNode } from "react";
import { ContributorPortal } from "@/components/contributor/portal";
import type { ContributorProfile, Me } from "@/lib/api/client";
import { serverGet } from "@/lib/api/server";

// The session and profile are read on the server so the first HTML already
// holds the portal (Stage 23, D-165). If either read fails, the portal loads
// them in the browser as before.
async function ServerPortal({ children }: { children: ReactNode }) {
  const [me, profile] = await Promise.all([
    serverGet<Me>("/v1/auth/me"),
    serverGet<ContributorProfile>("/v1/me/contributor"),
  ]);
  return (
    <ContributorPortal me={me} profile={profile}>
      {children}
    </ContributorPortal>
  );
}

const blank = <div className="min-h-dvh bg-bg" aria-busy="true" />;

export default function ContributorLayout({ children }: { children: ReactNode }) {
  return (
    <Suspense fallback={blank}>
      <ServerPortal>{children}</ServerPortal>
    </Suspense>
  );
}
