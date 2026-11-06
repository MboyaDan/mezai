import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { Organisations } from "@/components/workspace/organisations";

export const metadata: Metadata = { title: "Organisations", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Organisations />
    </Suspense>
  );
}
