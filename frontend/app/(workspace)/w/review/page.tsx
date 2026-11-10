import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ReviewQueue } from "@/components/workspace/review";

export const metadata: Metadata = { title: "Review", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ReviewQueue />
    </Suspense>
  );
}
