import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { Requests } from "@/components/workspace/requests";

export const metadata: Metadata = { title: "Project requests", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Requests />
    </Suspense>
  );
}
