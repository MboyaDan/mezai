/**
 * Cohen's kappa for two raters from a square count matrix (rows: rater A,
 * columns: rater B). Used by the explainer on /quality; the quality engine
 * (Stage 10) computes project figures in the backend.
 *
 *   kappa = (po − pe) / (1 − pe)
 *
 * po is the observed share of agreement, pe the agreement expected by chance
 * from each rater's label frequencies. Returns null when undefined: no items,
 * or pe = 1 (both raters always use one and the same label).
 */
export function cohensKappa(
  matrix: readonly (readonly number[])[],
): { po: number; pe: number; kappa: number } | null {
  const k = matrix.length;
  if (k === 0 || matrix.some((row) => row.length !== k)) return null;
  let n = 0;
  let agreed = 0;
  const rowSum = new Array<number>(k).fill(0);
  const colSum = new Array<number>(k).fill(0);
  matrix.forEach((row, i) =>
    row.forEach((v, j) => {
      n += v;
      rowSum[i] += v;
      colSum[j] += v;
      if (i === j) agreed += v;
    }),
  );
  if (n === 0) return null;
  const po = agreed / n;
  const pe = rowSum.reduce((s, r, i) => s + (r / n) * (colSum[i] / n), 0);
  if (pe === 1) return null;
  return { po, pe, kappa: (po - pe) / (1 - pe) };
}

/**
 * A 2×2 example for the /quality explainer: `total` items, of which
 * `agreements` are labelled the same by both raters, a `positiveShare` of
 * those as the first label. Disagreements split as evenly as possible.
 */
export function twoRaterExample(total: number, agreements: number, positiveShare: number): number[][] {
  const a = Math.max(0, Math.min(total, Math.round(agreements)));
  const both = Math.round(a * Math.max(0, Math.min(1, positiveShare)));
  const off = total - a;
  const aOnly = Math.ceil(off / 2);
  return [
    [both, aOnly],
    [off - aOnly, a - both],
  ];
}
