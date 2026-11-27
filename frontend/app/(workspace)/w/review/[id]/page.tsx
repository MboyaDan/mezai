import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ReviewItemView } from "@/components/workspace/review";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";

export const metadata: Metadata = { title: "Review item", robots: { index: false } };

// The item with every answer, read on the server (Stage 27, D-169). The
// decision itself is a POST and stays in the browser.
async function Preloaded({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return (
    <PreloadProvider data={await serverPreload([`/v1/review/items/${id}`])}>
      <ReviewItemView />
    </PreloadProvider>
  );
}

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <Preloaded params={params} />
    </Suspense>
  );
}
