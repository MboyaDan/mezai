import type { Metadata } from "next";
import { Suspense } from "react";
import { Contributors } from "@/components/workspace/contributors";
import { Skeleton } from "@/components/ui/display";
import { meCan, serverMe, serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";
import { contributorsTab, contributorsTabPath } from "@/lib/workspace-reads";

export const metadata: Metadata = { title: "Contributors", robots: { index: false } };

type Search = Promise<Record<string, string | string[] | undefined>>;

// The tab the page opens on, read on the server (Stage 27, D-169). The same
// rule picks the tab in the browser, so the data matches what is shown.
async function Preloaded({ searchParams }: { searchParams: Search }) {
  const [me, q] = await Promise.all([serverMe(), searchParams]);
  const tab = contributorsTab(
    Boolean(me && meCan(me, "contributor.approve")),
    typeof q.tab === "string" ? q.tab : null,
  );
  const path = me ? contributorsTabPath(tab) : null;
  return (
    <PreloadProvider data={path ? await serverPreload([path]) : {}}>
      <Contributors />
    </PreloadProvider>
  );
}

export default function Page({ searchParams }: { searchParams: Search }) {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Preloaded searchParams={searchParams} />
    </Suspense>
  );
}
