// Contributor dashboard helpers (Stage 12b): plain words for the trust
// score, earnings and statement. Numbers come from the API unchanged
// (D-119); these functions only format and explain them.

import type {
  EarningsSummary,
  WorkProject,
  EarningsTotals,
  MyQuality,
  Payout,
  StatementLine,
} from "@/lib/api/client";
import { formatKes } from "@/lib/projects";

/** A share 0..1 as a whole percent ("84%"), or an en dash when there is no data. */
export function percent(v: number | null | undefined): string {
  return v == null ? "–" : `${Math.round(v * 100)}%`;
}

/** The trust score with two decimals ("0.86"), or an en dash. */
export function score(v: number | null | undefined): string {
  return v == null ? "–" : v.toFixed(2);
}

/** The totals to show: the pilot currency (KES) first, or an all-zero row. */
export function mainTotals(s: EarningsSummary): EarningsTotals {
  return (
    s.totals.find((t) => t.currency === "KES") ??
    s.totals[0] ?? {
      currency: "KES",
      pending_cents: 0,
      pending_tasks: 0,
      approved_cents: 0,
      paid_cents: 0,
      earned_cents: 0,
    }
  );
}

/** Signed money for statement lines: "+ KES 100.00", "− KES 60.00". */
export function signedKes(cents: number, currency = "KES"): string {
  return `${cents < 0 ? "−" : "+"} ${formatKes(Math.abs(cents), currency)}`;
}

/** "4 Dec 2026" from a YYYY-MM-DD date, read as a calendar day (no time zone shift). */
export function formatDay(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  return new Date(Date.UTC(y, m - 1, d)).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  });
}

/** What a statement line is, in the contributor's words. */
export function lineLabel(l: StatementLine): string {
  switch (l.type) {
    case "accrual":
      return `${l.tasks} ${l.tasks === 1 ? "task" : "tasks"} accepted`;
    case "payout":
      return "Paid to M-Pesa";
    case "payout_reversal":
      return "Payment returned";
    default:
      return l.amount_cents < 0 ? "Correction (deducted)" : "Correction (added)";
  }
}

export const payoutTone: Record<Payout["status"], "success" | "danger" | "info"> = {
  paid: "success",
  failed: "danger",
  processing: "info",
};

export const payoutLabel: Record<Payout["status"], string> = {
  paid: "Paid",
  failed: "Not paid",
  processing: "Processing",
};

/** The next-payout sentence for the earnings card, or null to show nothing. */
export function payoutNote(s: EarningsSummary): string | null {
  const min = formatKes(s.min_payout_cents);
  if (s.next_payout_date)
    return `Next payout ${formatDay(s.next_payout_date)}. Balances of ${min} or more are paid.`;
  return `Paid ${s.payout_schedule} by M-Pesa once your approved balance reaches ${min}.`;
}

export type TrustState = "none" | "building" | "good" | "low";

/** Where the score stands against the floor. */
export function trustState(q: MyQuality): TrustState {
  const t = q.trust;
  if (!t || t.score == null) return "none";
  if (t.building) return "building";
  return t.score >= q.floor ? "good" : "low";
}

/** One or two plain sentences on what the score means for the contributor now. */
export function trustExplanation(q: MyQuality): string {
  const floor = score(q.floor);
  switch (trustState(q)) {
    case "none":
      return "Your score appears the day after your first scored work. It combines how often you match known answers, how often you agree with the final answer and how often reviewers accept your work.";
    case "building":
      return `Your score is still building: it starts to count once you have done ${q.min_gold_results} checks with known answers. Until then it cannot pause your account.`;
    case "good":
      return `You are above the ${floor} floor. Keep reading each task carefully; the score follows your last ${q.window_days} days of work.`;
    default:
      return `Your score is below the ${floor} floor. That pauses new tasks until the MezAI team has looked at your recent work; reread the project guidelines and your latest feedback.`;
  }
}

/** The three inputs with their weights, in the brief's order (brief 8.4). */
export function trustInputs(q: MyQuality) {
  const t = q.trust;
  return [
    {
      key: "gold",
      label: "Gold accuracy",
      hint: "Tasks with a known answer that you matched",
      value: t?.gold_accuracy ?? null,
      weight: q.weights.gold_accuracy,
    },
    {
      key: "agreement",
      label: "Agreement with the final answer",
      hint: "How often your answer matched the agreed one",
      value: t?.agreement ?? null,
      weight: q.weights.agreement,
    },
    {
      key: "review",
      label: "Review acceptance",
      hint: "Work a reviewer checked and accepted",
      value: t?.review_acceptance ?? null,
      weight: q.weights.review_acceptance,
    },
  ];
}

/** Gold accuracy trend points for the line chart: days since the first point. */
export function goldTrend(q: MyQuality): { x: number; y: number; day: string }[] {
  const pts = q.trend.filter((p) => p.gold_accuracy != null);
  if (pts.length === 0) return [];
  const t0 = Date.parse(`${pts[0].day}T00:00:00Z`);
  return pts.map((p) => ({
    x: Math.round((Date.parse(`${p.day}T00:00:00Z`) - t0) / 86_400_000),
    y: p.gold_accuracy as number,
    day: p.day,
  }));
}

/** "about 12 tasks left", "no tasks left for you", or null when unknown. */
export function tasksLeftText(n: number | undefined): string | null {
  if (n == null) return null;
  if (n === 0) return "no tasks left for you";
  return `about ${n.toLocaleString("en-KE")} ${n === 1 ? "task" : "tasks"} left`;
}

/** "about 1.5 min a task", or null until there are enough answers. */
export function minutesText(m: number | undefined): string | null {
  if (m == null) return null;
  return `about ${m % 1 === 0 ? m : m.toFixed(1)} min a task`;
}

/** Dashboard rows: projects that still have work for the caller, at most `max`. */
export function projectsWithWork(list: WorkProject[], max = 5) {
  const open = list.filter((p) => p.tasks_left !== 0);
  return { shown: open.slice(0, max), open: open.length };
}
