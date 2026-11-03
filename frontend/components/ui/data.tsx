import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { agreementBand } from "@/lib/agreement";

/* Stat: a label over a mono number. Only real records or labelled samples. */
export function Stat({
  label,
  value,
  unit,
  caption,
  size = "md",
  className,
}: {
  label: ReactNode;
  value: ReactNode;
  unit?: ReactNode;
  caption?: ReactNode;
  size?: "md" | "lg";
  className?: string;
}) {
  return (
    <div className={cn("grid content-start gap-1", className)}>
      <dt className="text-sm text-fg-muted">{label}</dt>
      <dd className="flex items-baseline gap-1.5">
        <span
          className={cn(
            "font-mono font-medium tracking-tight text-fg tabular",
            size === "lg" ? "text-5xl max-md:text-4xl" : "text-[1.75rem] leading-tight",
          )}
        >
          {value}
        </span>
        {unit && <span className="text-sm text-fg-muted">{unit}</span>}
      </dd>
      {caption && <dd className="text-sm text-fg-muted">{caption}</dd>}
    </div>
  );
}

/* StatGroup: the <dl> that Stat items belong to; a KPI row. */
export function StatGroup({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <dl
      className={cn(
        "grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-[repeat(auto-fit,minmax(9rem,1fr))]",
        className,
      )}
    >
      {children}
    </dl>
  );
}

/* ProgressBar: determinate only. The value is always written out as text. */
export function ProgressBar({
  label,
  value,
  max = 100,
  valueText,
  className,
}: {
  label: string;
  value: number;
  max?: number;
  /** Visible and announced text, e.g. "1,240 of 2,000 items". Defaults to a percentage. */
  valueText?: string;
  className?: string;
}) {
  const ratio = max > 0 ? Math.min(Math.max(value / max, 0), 1) : 0;
  const text = valueText ?? `${Math.round(ratio * 100)}%`;
  return (
    <div className={cn("grid gap-1.5", className)}>
      <div className="flex items-baseline justify-between gap-3 text-sm">
        <span className="text-fg">{label}</span>
        <span className="font-mono text-fg-muted tabular">{text}</span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={max}
        aria-valuenow={value}
        aria-valuetext={text}
        className="h-2 overflow-hidden rounded-full bg-border"
      >
        <div
          className="h-full rounded-full bg-accent-fg transition-[width] duration-(--dur-base) ease-out"
          style={{ width: `${ratio * 100}%` }}
        />
      </div>
    </div>
  );
}

/* Agreement: the number and its plain-language band, never one without the other. */
export function Agreement({
  value,
  statistic = "κ",
  className,
}: {
  value: number;
  statistic?: string;
  className?: string;
}) {
  return (
    <span className={cn("inline-flex items-baseline gap-1.5", className)}>
      <span className="font-mono text-fg tabular">
        {statistic && `${statistic} `}
        {value.toFixed(2)}
      </span>
      <span className="font-sans text-sm text-fg-muted">{agreementBand(value)}</span>
    </span>
  );
}
