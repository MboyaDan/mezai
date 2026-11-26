import type { Metadata } from "next";
import { Suspense } from "react";
import { Skeleton } from "@/components/ui/display";
import { EnquiryDetail } from "@/components/workspace/enquiries";

export const metadata: Metadata = { title: "Enquiry", robots: { index: false } };

export default function Page() {
  return (
    <Suspense fallback={<Skeleton className="h-80 w-full" />}>
      <EnquiryDetail />
    </Suspense>
  );
}
