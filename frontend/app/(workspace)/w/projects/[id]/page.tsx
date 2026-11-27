import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { ProjectOverview } from "@/components/workspace/projects/overview";
import type { Project } from "@/lib/api/client";
import { serverGet, serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";
import { sees, type Panel } from "@/lib/visibility";

export const metadata: Metadata = { title: "Project", robots: { index: false } };

// The overview tab's panels, read on the server in parallel (Stage 26, D-168).
// The project itself comes from the layout's read (one per request). Panels
// Ops holds back from a client (D-167) are not asked for.
async function Preloaded({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const base = `/v1/projects/${id}` as const;
  const project = await serverGet<Project>(base);
  const shows = (panel: Panel) => !project || sees(project, panel);
  const data = await serverPreload([
    `${base}/progress`,
    ...(shows("quality") ? [`${base}/quality` as const] : []),
    ...(shows("workforce") ? [`${base}/workforce` as const] : []),
    `${base}/imports`,
    `${base}/retention`,
  ]);
  return (
    <PreloadProvider data={data}>
      <ProjectOverview />
    </PreloadProvider>
  );
}

export default function Page({ params }: { params: Promise<{ id: string }> }) {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Preloaded params={params} />
    </Suspense>
  );
}
