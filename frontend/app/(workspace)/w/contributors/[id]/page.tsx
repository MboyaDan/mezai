import type { Metadata } from "next";
import { Suspense } from "react";
import { ContributorDetail } from "@/components/workspace/contributor-detail";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Contributor", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <ContributorDetail />
    </Suspense>
  );
}
