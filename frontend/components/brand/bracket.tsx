import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

/**
 * The annotation bracket: two corner marks, like a bounding box, framing a
 * key element (DESIGN.md section 8). Decorative.
 */
export function Bracket({
  children,
  className,
  tone = "fg",
}: {
  children: ReactNode;
  className?: string;
  tone?: "fg" | "accent";
}) {
  const mark = cn(
    "pointer-events-none absolute size-3.5",
    tone === "accent" ? "border-accent-fg" : "border-fg",
  );
  return (
    <div className={cn("relative", className)}>
      <span aria-hidden="true" className={cn(mark, "-top-2.5 -left-2.5 border-t border-l")} />
      {children}
      <span aria-hidden="true" className={cn(mark, "-right-2.5 -bottom-2.5 border-r border-b")} />
    </div>
  );
}
