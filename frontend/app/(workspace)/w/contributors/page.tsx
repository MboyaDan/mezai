import type { Metadata } from "next";
import { Suspense } from "react";
import { Contributors } from "@/components/workspace/contributors";
import { Skeleton } from "@/components/ui/display";

export const metadata: Metadata = { title: "Contributors", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <Contributors />
    </Suspense>
  );
}
