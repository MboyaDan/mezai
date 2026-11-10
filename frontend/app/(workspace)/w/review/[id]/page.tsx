import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ReviewItemView } from "@/components/workspace/review";

export const metadata: Metadata = { title: "Review item", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full" />}>
      <ReviewItemView />
    </Suspense>
  );
}
