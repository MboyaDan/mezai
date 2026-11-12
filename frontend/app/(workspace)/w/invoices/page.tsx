import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { InvoicesPage } from "@/components/workspace/invoices";

export const metadata: Metadata = { title: "Invoices", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <InvoicesPage />
    </Suspense>
  );
}
