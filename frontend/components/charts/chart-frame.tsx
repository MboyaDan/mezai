import type { ReactNode } from "react";
import { cn } from "@/lib/cn";

export type Series = 1 | 2 | 3 | 4 | 5;

/** Tailwind needs whole class names, so series colours are looked up, not built. */
export const seriesBg: Record<Series, string> = {
  1: "bg-series-1",
  2: "bg-series-2",
  3: "bg-series-3",
  4: "bg-series-4",
  5: "bg-series-5",
};
export const seriesVar = (s: Series) => `var(--series-${s})`;

/** Figure with a visible title and an optional note such as "Demonstration data". */
export function ChartFrame({
  title,
  note,
  children,
  className,
}: {
  title: ReactNode;
  note?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <figure className={cn("grid gap-4", className)}>
      <figcaption className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <span className="font-display text-base font-semibold text-fg">{title}</span>
        {note && <span className="text-xs text-fg-muted">{note}</span>}
      </figcaption>
      {children}
    </figure>
  );
}
