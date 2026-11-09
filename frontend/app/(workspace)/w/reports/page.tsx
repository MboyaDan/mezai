import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { Reports } from "@/components/workspace/reports";

export const metadata: Metadata = { title: "Content reports", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Reports />
    </Suspense>
  );
}
