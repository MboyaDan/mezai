import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { NewRequest } from "@/components/workspace/requests";

export const metadata: Metadata = { title: "Request a project", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <NewRequest />
    </Suspense>
  );
}
