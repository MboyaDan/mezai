import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectOverview } from "@/components/workspace/projects/overview";

export const metadata: Metadata = { title: "Project", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ProjectOverview />
    </Suspense>
  );
}
