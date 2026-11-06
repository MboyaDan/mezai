import type { Metadata } from "next";
import { Overview } from "@/components/workspace/overview";

export const metadata: Metadata = { title: "Workspace", robots: { index: false } };

export default function Page() {
  return <Overview />;
}
