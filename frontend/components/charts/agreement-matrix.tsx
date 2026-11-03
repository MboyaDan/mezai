import { cn } from "@/lib/cn";
import { percentAgreement } from "@/lib/agreement";
import { Agreement } from "@/components/ui/data";
import { ChartFrame } from "./chart-frame";

/**
 * Agreement (confusion) matrix between two raters, or a rater and consensus.
 * Rows are rater A's labels, columns rater B's. The diagonal is agreement.
 * Tints stay at or under 35 percent so cell text keeps its contrast on both themes.
 */
export function AgreementMatrix({
  title,
  note,
  labels,
  counts,
  rowRater = "Rater A",
  columnRater = "Rater B",
  kappa,
  className,
}: {
  title: string;
  note?: string;
  labels: string[];
  counts: number[][];
  rowRater?: string;
  columnRater?: string;
  /** Chance-corrected statistic from the quality engine; the UI never computes it. */
  kappa?: { value: number; name: string };
  className?: string;
}) {
  const share = percentAgreement(counts);
  // Diagonal and off-diagonal scale separately, so disagreement hot spots
  // show even when agreement counts are much larger.
  const cells = counts.flatMap((row, i) => row.map((n, j) => ({ n, agree: i === j })));
  const maxAgree = Math.max(0, ...cells.filter((c) => c.agree).map((c) => c.n));
  const maxDisagree = Math.max(0, ...cells.filter((c) => !c.agree).map((c) => c.n));
  const tint = (n: number, agree: boolean) => {
    const max = agree ? maxAgree : maxDisagree;
    if (max === 0 || n === 0) return undefined;
    const pct = Math.round((n / max) * (agree ? 35 : 25));
    return `color-mix(in srgb, var(${agree ? "--cell-agree" : "--cell-disagree"}) ${pct}%, transparent)`;
  };

  return (
    <ChartFrame title={title} note={note} className={className}>
      <div role="region" aria-label={title} tabIndex={0} className="overflow-x-auto">
        <table className="border-collapse text-sm">
          <caption className="sr-only">
            {title}. Rows: {rowRater}. Columns: {columnRater}. Cells on the diagonal are agreements.
          </caption>
          <thead>
            <tr>
              <td className="p-2" />
              <th
                scope="colgroup"
                colSpan={labels.length}
                className="px-2 pb-1 text-left text-xs font-semibold text-fg-muted"
              >
                {columnRater} →
              </th>
            </tr>
            <tr>
              <th scope="col" className="px-2 pb-2 text-left text-xs font-semibold text-fg-muted">
                {rowRater} ↓
              </th>
              {labels.map((l) => (
                <th key={l} scope="col" className="min-w-16 px-2 pb-2 text-right text-xs font-medium text-fg">
                  {l}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {labels.map((rowLabel, i) => (
              <tr key={rowLabel}>
                <th
                  scope="row"
                  className="py-2 pr-4 pl-2 text-left text-xs font-medium whitespace-nowrap text-fg"
                >
                  {rowLabel}
                </th>
                {labels.map((_, j) => (
                  <td
                    key={j}
                    className={cn(
                      "border border-border px-3 py-2.5 text-right font-mono tabular",
                      i === j ? "font-semibold text-fg" : "text-fg-muted",
                    )}
                    style={{ background: tint(counts[i]?.[j] ?? 0, i === j) }}
                  >
                    {counts[i]?.[j] ?? 0}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <dl className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
        <div className="flex items-baseline gap-2">
          <dt className="text-fg-muted">Percent agreement</dt>
          <dd className="font-mono text-fg tabular">
            {share === null ? "No items" : `${(share * 100).toFixed(1)}%`}
          </dd>
        </div>
        {kappa && (
          <div className="flex items-baseline gap-2">
            <dt className="text-fg-muted">{kappa.name}</dt>
            <dd>
              <Agreement value={kappa.value} statistic="" />
            </dd>
          </div>
        )}
      </dl>
    </ChartFrame>
  );
}
