import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { Projects } from "@/components/workspace/projects/list";

export const metadata: Metadata = { title: "Projects", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Projects />
    </Suspense>
  );
}
