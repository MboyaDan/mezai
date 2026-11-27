import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { PayoutBatchPage } from "@/components/workspace/payout-batch";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";

export const metadata: Metadata = { title: "Payout batch", robots: { index: false } };

// The batch and its payment states, read on the server in parallel (Stage
// 27, D-169). The page uses them only when both arrived; approving, paying
// and stopping lines stay in the browser.
async function Preloaded({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const base = `/v1/payouts/batches/${id}` as const;
  return (
    <PreloadProvider data={await serverPreload([base, `${base}/transactions`])}>
      <PayoutBatchPage />
    </PreloadProvider>
  );
}

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Preloaded params={params} />
    </Suspense>
  );
}
