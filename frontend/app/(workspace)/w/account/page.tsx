import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { AccountPage } from "@/components/workspace/two-step";

export const metadata: Metadata = { title: "Account", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <AccountPage />
    </Suspense>
  );
}
