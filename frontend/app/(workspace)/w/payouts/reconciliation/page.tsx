import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ReconciliationPage } from "@/components/workspace/payout-reconciliation";

export const metadata: Metadata = { title: "M-Pesa reconciliation", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ReconciliationPage />
    </Suspense>
  );
}
