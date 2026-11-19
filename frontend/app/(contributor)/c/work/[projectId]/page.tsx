import type { Metadata } from "next";
import { Suspense } from "react";
import { TaskWorkspace } from "@/components/contributor/task-workspace";
import { Skeleton } from "@/components/ui/display";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";

export const metadata: Metadata = { title: "Task", robots: { index: false } };

// The project (name, instructions, settings) is read on the server (D-165).
// Reserving the next task is a write, so it stays in the browser with the
// CSRF token and never runs during rendering or prefetch.
async function Preloaded({ params }: { params: Promise<{ projectId: string }> }) {
  const { projectId } = await params;
  const data = /^[0-9a-f-]{36}$/i.test(projectId)
    ? await serverPreload([`/v1/me/projects/${projectId}`])
    : {};
  return (
    <PreloadProvider data={data}>
      <TaskWorkspace />
    </PreloadProvider>
  );
}

export default function Page({ params }: { params: Promise<{ projectId: string }> }) {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Preloaded params={params} />
    </Suspense>
  );
}
