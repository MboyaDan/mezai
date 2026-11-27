import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ReviewQueue } from "@/components/workspace/review";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";
import { queuePath } from "@/lib/workspace-reads";

export const metadata: Metadata = { title: "Review", robots: { index: false } };

type Search = Promise<Record<string, string | string[] | undefined>>;
const one = (v: string | string[] | undefined) => (typeof v === "string" ? v : "");

// The queue for the page's filters, read on the server (Stage 27, D-169).
async function Preloaded({ searchParams }: { searchParams: Search }) {
  const q = await searchParams;
  return (
    <PreloadProvider data={await serverPreload([queuePath(one(q.project), one(q.reason))])}>
      <ReviewQueue />
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
