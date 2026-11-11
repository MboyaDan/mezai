import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ActivityPage } from "@/components/workspace/activity";

export const metadata: Metadata = { title: "Activity", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ActivityPage />
    </Suspense>
  );
}
