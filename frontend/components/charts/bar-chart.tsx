import { cn } from "@/lib/cn";
import { ChartFrame, seriesBg, type Series } from "./chart-frame";

type Bar = { label: string; value: number };

/**
 * Horizontal bars with the label and value written beside each bar, so the
 * chart reads as text too (bars are decoration for screen readers).
 */
export function BarChart({
  title,
  note,
  data,
  max,
  format = String,
  series = 1,
  reference,
  className,
}: {
  title: string;
  note?: string;
  data: Bar[];
  /** Scale maximum; defaults to the largest value. Use 1 for ratios. */
  max?: number;
  format?: (v: number) => string;
  series?: Series;
  /** A threshold drawn across every bar, e.g. the review cut-off. */
  reference?: { value: number; label: string };
  className?: string;
}) {
  const top = max ?? Math.max(...data.map((d) => d.value), 0);
  const pct = (v: number) => (top > 0 ? Math.min(Math.max(v / top, 0), 1) * 100 : 0);

  return (
    <ChartFrame title={title} note={note} className={className}>
      <ul className="grid gap-3">
        {data.map((d) => (
          <li
            key={d.label}
            className="grid grid-cols-[1fr_auto] items-center gap-x-4 gap-y-1.5 sm:grid-cols-[minmax(6rem,10rem)_1fr_4rem]"
          >
            <span className="truncate text-sm text-fg" title={d.label}>
              {d.label}
            </span>
            <span className="text-right font-mono text-sm text-fg tabular sm:order-last">
              {format(d.value)}
            </span>
            <span aria-hidden="true" className="relative col-span-2 h-3 bg-sunken sm:col-span-1">
              <span
                className={cn("chart-bar absolute inset-y-0 left-0", seriesBg[series])}
                style={{ width: `${pct(d.value)}%` }}
              />
              {reference && (
                <span
                  className="absolute -inset-y-1 border-l border-dashed border-fg"
                  style={{ left: `${pct(reference.value)}%` }}
                />
              )}
            </span>
          </li>
        ))}
      </ul>
      {reference && (
        <p className="flex items-center gap-2 text-xs text-fg-muted">
          <span aria-hidden="true" className="h-3 border-l border-dashed border-fg" />
          {reference.label}: <span className="font-mono tabular">{format(reference.value)}</span>
        </p>
      )}
    </ChartFrame>
  );
}
