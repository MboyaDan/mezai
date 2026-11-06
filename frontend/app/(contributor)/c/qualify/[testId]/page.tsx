import type { Metadata } from "next";
import { Suspense } from "react";
import { TakeTest } from "@/components/contributor/take-test";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Qualification test", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <TakeTest />
    </Suspense>
  );
}
