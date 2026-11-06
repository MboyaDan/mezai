import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectGold } from "@/components/workspace/projects/gold";

export const metadata: Metadata = { title: "Gold set", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ProjectGold />
    </Suspense>
  );
}
