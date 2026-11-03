import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { MotionProvider } from "@/motion/provider";

export const metadata: Metadata = {
  title: "Design system",
  robots: { index: false, follow: false },
};

// Dev-only gallery. Production builds return 404 unless ENABLE_DESIGN_PAGE=1
// (set in CI so the axe checks can run against the production build).
export default function DesignLayout({ children }: LayoutProps<"/design">) {
  if (process.env.NODE_ENV === "production" && process.env.ENABLE_DESIGN_PAGE !== "1") notFound();
  return <MotionProvider>{children}</MotionProvider>;
}
