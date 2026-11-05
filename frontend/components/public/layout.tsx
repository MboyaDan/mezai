import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

/** Page width and gutters shared by every public section. */
export function Container({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn("mx-auto w-full max-w-[82rem] px-4 sm:px-8 lg:px-16", className)}>{children}</div>
  );
}

/** Small mono label above a section title. */
export function Eyebrow({ children, className }: { children: ReactNode; className?: string }) {
  return <p className={cn("font-mono text-xs tracking-wide text-fg-muted", className)}>{children}</p>;
}

/** Section title at display size; the serif accent goes in <Accent>. */
export function SectionTitle({
  id,
  children,
  className,
}: {
  id: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <h2
      id={id}
      className={cn(
        "font-display text-[2.25rem] leading-[1.05] font-medium tracking-[-0.02em] md:text-display-3 lg:text-display-2",
        className,
      )}
    >
      {children}
    </h2>
  );
}

export function Accent({ children }: { children: ReactNode }) {
  return <span className="font-serif font-normal tracking-normal text-serif-accent italic">{children}</span>;
}
