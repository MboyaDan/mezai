import type { Metadata } from "next";
import { Suspense } from "react";
import { TaskWorkspace } from "@/components/contributor/task-workspace";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Task", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <TaskWorkspace />
    </Suspense>
  );
}
