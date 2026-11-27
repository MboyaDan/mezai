import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { PayoutsPage } from "@/components/workspace/payouts";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";

export const metadata: Metadata = { title: "Payouts", robots: { index: false } };

// The batch list, read on the server (Stage 27, D-169).
async function Preloaded() {
  return (
    <PreloadProvider data={await serverPreload(["/v1/payouts/batches"])}>
      <PayoutsPage />
    </PreloadProvider>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Preloaded />
    </Suspense>
  );
}
