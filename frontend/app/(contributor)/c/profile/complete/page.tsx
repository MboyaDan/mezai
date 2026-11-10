import type { Metadata } from "next";
import { ApplicationReceived } from "@/components/contributor/application-received";

export const metadata: Metadata = { title: "Application received", robots: { index: false } };

export default function Page() {
  return <ApplicationReceived />;
}
