import { cn } from "@/lib/cn";
import { linearScale, niceTicks } from "@/lib/charts/scale";
import { ChartFrame, seriesBg, seriesVar, type Series } from "./chart-frame";

type Point = { x: number; y: number };
type Line = { name: string; points: Point[]; series?: Series };

/**
 * Line chart. Lines are drawn in a stretched 100 x 100 SVG with
 * non-scaling strokes; labels and end dots are HTML placed by percentage, so
 * the chart fills any width without measuring and stays legible at 360 px.
 */
export function LineChart({
  title,
  note,
  lines,
  formatX = String,
  formatY = String,
  yDomain,
  height = 200,
  className,
}: {
  title: string;
  note?: string;
  lines: Line[];
  formatX?: (x: number) => string;
  formatY?: (y: number) => string;
  /** Fixed y range, e.g. [0, 1] for accuracy. Defaults to round ticks around the data. */
  yDomain?: [number, number];
  height?: number;
  className?: string;
}) {
  const all = lines.flatMap((l) => l.points);
  if (all.length === 0) return null;
  const xs = all.map((p) => p.x);
  const ys = all.map((p) => p.y);
  const yTicks = yDomain
    ? niceTicks(yDomain[0], yDomain[1], 4)
    : niceTicks(Math.min(0, ...ys), Math.max(...ys), 4);
  const [y0, y1] = yDomain ?? [yTicks[0], yTicks[yTicks.length - 1]];
  const xMin = Math.min(...xs);
  const xMax = Math.max(...xs);
  const sx = linearScale([xMin, xMax], [0, 100]);
  const sy = linearScale([y0, y1], [100, 0]);
  // First, middle and last data points, so labels name real points (no "Day 5.5").
  const uniqueX = [...new Set(xs)].sort((a, b) => a - b);
  const xTicks =
    uniqueX.length < 3
      ? uniqueX
      : [uniqueX[0], uniqueX[Math.floor((uniqueX.length - 1) / 2)], uniqueX[uniqueX.length - 1]];
  const series = (l: Line, i: number): Series => l.series ?? (((i % 5) + 1) as Series);

  return (
    <ChartFrame title={title} note={note} className={className}>
      <div className="grid grid-cols-[auto_1fr] gap-x-3" aria-hidden="true">
        <div className="relative w-10" style={{ height }}>
          {yTicks
            .filter((t) => t >= y0 && t <= y1)
            .map((t) => (
              <span
                key={t}
                className="absolute right-0 -translate-y-1/2 font-mono text-[0.6875rem] text-fg-muted tabular"
                style={{ top: `${sy(t)}%` }}
              >
                {formatY(t)}
              </span>
            ))}
        </div>
        <div className="relative" style={{ height }}>
          <svg
            viewBox="0 0 100 100"
            preserveAspectRatio="none"
            className="absolute inset-0 size-full overflow-visible"
          >
            {yTicks
              .filter((t) => t >= y0 && t <= y1)
              .map((t) => (
                <line
                  key={t}
                  x1={0}
                  x2={100}
                  y1={sy(t)}
                  y2={sy(t)}
                  vectorEffect="non-scaling-stroke"
                  className="stroke-border"
                  strokeWidth={1}
                />
              ))}
            {lines.map((l, i) => (
              <path
                key={l.name}
                d={l.points.map((p, j) => `${j ? "L" : "M"}${sx(p.x)},${sy(p.y)}`).join("")}
                fill="none"
                stroke={seriesVar(series(l, i))}
                strokeWidth={2}
                strokeLinejoin="round"
                strokeLinecap="round"
                vectorEffect="non-scaling-stroke"
                className="chart-line"
              />
            ))}
          </svg>
          {lines.map((l, i) => {
            const last = l.points[l.points.length - 1];
            return (
              last && (
                <span
                  key={l.name}
                  className={cn(
                    "absolute size-2 -translate-1/2 rounded-full ring-2 ring-surface",
                    seriesBg[series(l, i)],
                  )}
                  style={{ left: `${sx(last.x)}%`, top: `${sy(last.y)}%` }}
                />
              )
            );
          })}
        </div>
        <div />
        <div className="relative mt-2 h-4">
          {xTicks.map((t, i) => (
            <span
              key={t}
              className={cn(
                "absolute font-mono text-[0.6875rem] whitespace-nowrap text-fg-muted tabular",
                i === 0 ? "left-0" : i === xTicks.length - 1 ? "right-0" : "-translate-x-1/2",
              )}
              style={i > 0 && i < xTicks.length - 1 ? { left: `${sx(t)}%` } : undefined}
            >
              {formatX(t)}
            </span>
          ))}
        </div>
      </div>

      <ul className="flex flex-wrap gap-x-5 gap-y-1 text-sm">
        {lines.map((l, i) => {
          const last = l.points[l.points.length - 1];
          return (
            <li key={l.name} className="flex items-center gap-2">
              <span aria-hidden="true" className={cn("h-0.5 w-4", seriesBg[series(l, i)])} />
              <span className="text-fg">{l.name}</span>
              {last && <span className="font-mono text-fg-muted tabular">{formatY(last.y)}</span>}
            </li>
          );
        })}
      </ul>

      <details className="text-sm">
        <summary className="cursor-pointer text-fg-muted underline-offset-4 hover:text-fg hover:underline">
          Show data table
        </summary>
        <div className="mt-2 overflow-x-auto">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-border text-xs text-fg-muted">
                <th scope="col" className="py-1.5 pr-4 font-semibold">
                  Series
                </th>
                <th scope="col" className="py-1.5 pr-4 font-semibold">
                  Point
                </th>
                <th scope="col" className="py-1.5 text-right font-semibold">
                  Value
                </th>
              </tr>
            </thead>
            <tbody>
              {lines.flatMap((l) =>
                l.points.map((p) => (
                  <tr key={`${l.name}-${p.x}`} className="border-b border-border last:border-b-0">
                    <td className="py-1.5 pr-4">{l.name}</td>
                    <td className="py-1.5 pr-4 font-mono tabular">{formatX(p.x)}</td>
                    <td className="py-1.5 text-right font-mono tabular">{formatY(p.y)}</td>
                  </tr>
                )),
              )}
            </tbody>
          </table>
        </div>
      </details>
    </ChartFrame>
  );
}
