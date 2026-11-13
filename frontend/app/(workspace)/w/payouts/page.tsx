import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { PayoutsPage } from "@/components/workspace/payouts";

export const metadata: Metadata = { title: "Payouts", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <PayoutsPage />
    </Suspense>
  );
}
