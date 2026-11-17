import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { StaffPage } from "@/components/workspace/staff";

export const metadata: Metadata = { title: "Staff", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <StaffPage />
    </Suspense>
  );
}
