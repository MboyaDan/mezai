import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { TeamPage } from "@/components/workspace/team";

export const metadata: Metadata = { title: "Team", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <TeamPage />
    </Suspense>
  );
}
