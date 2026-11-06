import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectItems } from "@/components/workspace/projects/imports";

export const metadata: Metadata = { title: "Project items", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ProjectItems />
    </Suspense>
  );
}
