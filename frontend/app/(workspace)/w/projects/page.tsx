import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { Projects } from "@/components/workspace/projects/list";
import { serverPreload } from "@/lib/api/server";
import { PreloadProvider } from "@/lib/preload";

export const metadata: Metadata = { title: "Projects", robots: { index: false } };

// The unfiltered list, read on the server (Stage 26, D-168).
async function Preloaded() {
  return (
    <PreloadProvider data={await serverPreload(["/v1/projects"])}>
      <Projects />
    </PreloadProvider>
  );
}

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Preloaded />
    </Suspense>
  );
}
