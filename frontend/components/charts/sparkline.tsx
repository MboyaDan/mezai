import { linearScale } from "@/lib/charts/scale";
import { seriesVar, type Series } from "./chart-frame";

/** Inline trend with no axes. `label` must say what the trend shows, in words. */
export function Sparkline({
  values,
  label,
  width = 96,
  height = 28,
  series = 1,
  className,
}: {
  values: number[];
  label: string;
  width?: number;
  height?: number;
  series?: Series;
  className?: string;
}) {
  if (values.length === 0) return null;
  const pad = 3;
  const sx = linearScale([0, Math.max(values.length - 1, 1)], [pad, width - pad]);
  const sy = linearScale([Math.min(...values), Math.max(...values)], [height - pad, pad]);
  const d = values.map((v, i) => `${i ? "L" : "M"}${sx(i).toFixed(1)},${sy(v).toFixed(1)}`).join("");
  const lastI = values.length - 1;
  return (
    <svg
      role="img"
      aria-label={label}
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className={className}
    >
      <path
        d={d}
        fill="none"
        stroke={seriesVar(series)}
        strokeWidth={1.5}
        strokeLinejoin="round"
        strokeLinecap="round"
        className="chart-line"
      />
      <circle cx={sx(lastI)} cy={sy(values[lastI])} r={2.5} fill={seriesVar(series)} />
    </svg>
  );
}
