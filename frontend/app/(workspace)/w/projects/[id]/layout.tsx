import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectFrame } from "@/components/workspace/projects/frame";

export default function ProjectLayout({ children }: { children: React.ReactNode }) {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ProjectFrame>{children}</ProjectFrame>
    </Suspense>
  );
}
