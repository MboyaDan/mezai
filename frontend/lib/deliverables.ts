import type { DeliverableFile, DeliverableManifest } from "@/lib/api/client";

/** "1.2 MB" style size. */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v >= 10 ? Math.round(v) : Math.round(v * 10) / 10} ${units[i]}`;
}

export const statisticLabel: Record<string, string> = {
  percent: "Percent agreement",
  cohen_kappa: "Cohen's kappa",
  fleiss_kappa: "Fleiss' kappa",
  krippendorff_alpha: "Krippendorff's alpha",
  within_one: "Within one point",
  mean_abs_diff: "Mean score difference",
};

type Stat = DeliverableManifest["agreement"][number];

/** The whole-project figures, kappa first, then percent and score difference. */
export function headlineAgreement(stats: Stat[]): Stat[] {
  const order = ["fleiss_kappa", "cohen_kappa", "percent", "mean_abs_diff"];
  return stats
    .filter((s) => s.label == null)
    .sort((a, b) => order.indexOf(a.statistic) - order.indexOf(b.statistic));
}

export type CriterionAgreement = {
  criterion: string;
  alpha?: Stat;
  percent?: Stat;
  withinOne?: Stat;
  meanDiff?: Stat;
};

/**
 * Agreement for each rubric criterion (pairwise scores, translation adequacy
 * and fluency; D-163). Only labels with score figures are included, so
 * classification labels stay out.
 */
export function criterionAgreement(stats: Stat[]): CriterionAgreement[] {
  const rows = new Map<string, CriterionAgreement>();
  for (const s of stats) {
    if (s.label == null) continue;
    const row = rows.get(s.label) ?? { criterion: s.label };
    if (s.statistic === "krippendorff_alpha") row.alpha = s;
    else if (s.statistic === "percent") row.percent = s;
    else if (s.statistic === "within_one") row.withinOne = s;
    else if (s.statistic === "mean_abs_diff") row.meanDiff = s;
    rows.set(s.label, row);
  }
  return [...rows.values()].filter((r) => r.withinOne || r.meanDiff);
}

/** What the client receives, in one sentence. */
export function itemsSentence(items: DeliverableManifest["items"]): string {
  const n = (v: number) => v.toLocaleString("en-KE");
  const parts = [`${n(items.delivered)} of ${n(items.total)} items delivered`];
  if (items.rejected > 0) parts.push(`${n(items.rejected)} rejected at review and left out`);
  if (items.not_final > 0) parts.push(`${n(items.not_final)} still in progress`);
  return parts.join("; ") + ".";
}

/** Files in display order: JSONL, CSV, then the manifest. */
export function orderedFiles(files: DeliverableFile[]): DeliverableFile[] {
  const rank = (f: DeliverableFile) => (f.file === "manifest" ? 2 : f.format === "jsonl" ? 0 : 1);
  return [...files].sort((a, b) => rank(a) - rank(b));
}

export const fileLabel = (f: DeliverableFile) =>
  f.file === "manifest" ? "Manifest (JSON)" : f.format === "jsonl" ? "Answers (JSONL)" : "Answers (CSV)";
