import type { Metadata } from "next";
import { TestsList } from "@/components/contributor/tests-list";

export const metadata: Metadata = { title: "Qualification", robots: { index: false } };

export default function Page() {
  return <TestsList />;
}
