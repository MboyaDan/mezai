import type { Metadata } from "next";
import { Suspense } from "react";
import { JoinForm } from "@/components/auth/forms";

export const metadata: Metadata = { title: "Join as a contributor", robots: { index: false } };

export default function Page() {
  return (
    <Suspense>
      <JoinForm />
    </Suspense>
  );
}
