import type { Metadata } from "next";
import { Suspense } from "react";
import { Dashboard } from "@/components/contributor/dashboard";
import { Skeleton } from "@/components/ui/display";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";

export const metadata: Metadata = { title: "Dashboard", robots: { index: false } };

// Everything the dashboard shows, read on the server in parallel (D-165).
// Paths a contributor cannot read yet are simply left out.
async function Preloaded() {
  const data = await serverPreload([
    "/v1/me/qualification/tests",
    "/v1/me/projects",
    "/v1/me/quality",
    "/v1/me/earnings",
    "/v1/me/feedback",
  ]);
  return (
    <PreloadProvider data={data}>
      <Dashboard />
    </PreloadProvider>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-72 w-full" />}>
      <Preloaded />
    </Suspense>
  );
}
