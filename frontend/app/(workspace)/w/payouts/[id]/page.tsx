import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { PayoutBatchPage } from "@/components/workspace/payout-batch";

export const metadata: Metadata = { title: "Payout batch", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <PayoutBatchPage />
    </Suspense>
  );
}
