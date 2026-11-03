/**
 * Agreement interpretation bands from brief section 8.2:
 * below 0.40 weak, 0.41 to 0.60 moderate, 0.61 to 0.80 substantial,
 * above 0.80 strong. Boundary values fall in the lower band (A-013).
 * Bands are a guide; the UI always shows the number too.
 */
export type AgreementBand = "weak" | "moderate" | "substantial" | "strong";

export function agreementBand(kappa: number): AgreementBand {
  if (kappa <= 0.4) return "weak";
  if (kappa <= 0.6) return "moderate";
  if (kappa <= 0.8) return "substantial";
  return "strong";
}

/** Share of items on the diagonal of a square count matrix (rows: rater A, columns: rater B). */
export function percentAgreement(matrix: readonly (readonly number[])[]): number | null {
  let total = 0;
  let agreed = 0;
  matrix.forEach((row, i) =>
    row.forEach((n, j) => {
      total += n;
      if (i === j) agreed += n;
    }),
  );
  return total === 0 ? null : agreed / total;
}
