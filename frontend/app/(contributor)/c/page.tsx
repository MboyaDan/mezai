import type { Metadata } from "next";
import { Dashboard } from "@/components/contributor/dashboard";

export const metadata: Metadata = { title: "Dashboard", robots: { index: false } };

export default function Page() {
  return <Dashboard />;
}
