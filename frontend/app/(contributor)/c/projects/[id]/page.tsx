import type { Metadata } from "next";
import { Suspense } from "react";
import { ProjectBrief } from "@/components/contributor/work";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Project brief", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ProjectBrief />
    </Suspense>
  );
}
