import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { Overview } from "@/components/workspace/overview";
import type { Project } from "@/lib/api/client";
import { meCan, serverGet, serverMe, serverPreload, type Preloaded as Data } from "@/lib/api/server";
import { clientProjects, progressPath } from "@/lib/portal";
import { PreloadProvider } from "@/lib/preload";

export const metadata: Metadata = { title: "Workspace", robots: { index: false } };

// What this person's home shows, read on the server (Stage 26, D-168). The
// choice matches Overview: client home, Ops home or the reviewer's count.
async function homeData(): Promise<Data> {
  const me = await serverMe();
  if (!me) return {};
  if (me.user.kind === "client") {
    const list = await serverGet<{ projects: Project[] }>("/v1/projects");
    if (!list) return {};
    const shown = clientProjects(list.projects).filter((p) => p.status !== "draft");
    return { "/v1/projects": list, ...(await serverPreload(shown.map((p) => progressPath(p.id)))) };
  }
  if (meCan(me, "project.manage")) return serverPreload(["/v1/ops/overview"]);
  if (meCan(me, "review.read")) return serverPreload(["/v1/review/items"]);
  return {};
}

async function Preloaded() {
  return (
    <PreloadProvider data={await homeData()}>
      <Overview />
    </PreloadProvider>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-72 w-full" />}>
      <Preloaded />
    </Suspense>
  );
}
