import { Suspense, type ReactNode } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectFrame } from "@/components/workspace/projects/frame";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";

// The project, read on the server for every tab (Stage 26, D-168).
async function Preloaded({ params, children }: { params: Promise<{ id: string }>; children: ReactNode }) {
  const { id } = await params;
  return (
    <PreloadProvider data={await serverPreload([`/v1/projects/${id}`])}>
      <ProjectFrame>{children}</ProjectFrame>
    </PreloadProvider>
  );
}

export default function ProjectLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ id: string }>;
}) {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Preloaded params={params}>{children}</Preloaded>
    </Suspense>
  );
}
