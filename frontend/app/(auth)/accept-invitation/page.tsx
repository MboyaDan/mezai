import type { Metadata } from "next";
import { Suspense } from "react";
import { AcceptInvitationForm } from "@/components/auth/forms";

export const metadata: Metadata = { title: "Accept your invitation", robots: { index: false } };

export default function Page() {
  return (
    <Suspense>
      <AcceptInvitationForm />
    </Suspense>
  );
}
