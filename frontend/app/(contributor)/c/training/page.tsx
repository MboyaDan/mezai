import type { Metadata } from "next";
import { Suspense } from "react";
import { TrainingPage } from "@/components/contributor/training";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Training", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <TrainingPage />
    </Suspense>
  );
}
