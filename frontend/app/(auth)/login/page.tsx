import type { Metadata } from "next";
import { Suspense } from "react";
import { LoginForm } from "@/components/auth/forms";
import { serverGoogleEnabled } from "@/lib/api/server";

export const metadata: Metadata = { title: "Sign in", robots: { index: false } };

export default function Page() {
  // Not awaited: the form renders at once and the Google button streams in (D-171).
  const google = serverGoogleEnabled();
  return (
    <Suspense>
      <LoginForm google={google} />
    </Suspense>
  );
}
