import type { Metadata } from "next";
import { Suspense } from "react";
import { AttemptReview } from "@/components/workspace/attempt-review";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Review attempt", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <AttemptReview />
    </Suspense>
  );
}
