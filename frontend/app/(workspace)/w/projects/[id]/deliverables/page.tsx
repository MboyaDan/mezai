import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectDeliverables } from "@/components/workspace/projects/deliverables";

export const metadata: Metadata = { title: "Deliverables", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ProjectDeliverables />
    </Suspense>
  );
}
