import type { Metadata } from "next";
import { Suspense } from "react";
import { EarningsPage } from "@/components/contributor/earnings";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Earnings", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <EarningsPage />
    </Suspense>
  );
}
