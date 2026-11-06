import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectSettings } from "@/components/workspace/projects/pages";

export const metadata: Metadata = { title: "Project settings", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ProjectSettings />
    </Suspense>
  );
}
