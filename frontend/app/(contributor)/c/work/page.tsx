import type { Metadata } from "next";
import { Suspense } from "react";
import { WorkList } from "@/components/contributor/work";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Tasks", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <WorkList />
    </Suspense>
  );
}
