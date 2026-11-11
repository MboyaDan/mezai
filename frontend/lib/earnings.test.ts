import { describe, expect, it } from "vitest";
import type { EarningsSummary, MyQuality, StatementLine } from "@/lib/api/client";
import {
  formatDay,
  goldTrend,
  lineLabel,
  mainTotals,
  minutesText,
  payoutNote,
  percent,
  projectsWithWork,
  signedKes,
  tasksLeftText,
  trustExplanation,
  trustState,
} from "./earnings";

const quality = (
  trust: Partial<NonNullable<MyQuality["trust"]>> | null,
  trend: MyQuality["trend"] = [],
): MyQuality => ({
  as_of: "2026-11-11T00:00:00+03:00",
  trust: trust && {
    score: null,
    gold_accuracy: null,
    agreement: null,
    review_acceptance: null,
    building: false,
    calculated_at: "2026-11-10T20:00:00Z",
    ...trust,
  },
  trend,
  weights: { gold_accuracy: 0.5, agreement: 0.3, review_acceptance: 0.2 },
  floor: 0.7,
  window_days: 30,
  min_gold_results: 20,
});

const summary = (over: Partial<EarningsSummary> = {}): EarningsSummary => ({
  totals: [],
  tasks: { submitted: 0, waiting: 0, accepted: 0, not_accepted: 0 },
  next_payout_date: null,
  min_payout_cents: 50000,
  payout_schedule: "weekly",
  ...over,
});

const line = (over: Partial<StatementLine>): StatementLine => ({
  day: "2026-11-10",
  type: "accrual",
  currency: "KES",
  project_id: null,
  project_name: null,
  tasks: 0,
  amount_cents: 0,
  reason: null,
  mpesa_receipt: null,
  ...over,
});

describe("formatting", () => {
  it("shows shares, money and days plainly", () => {
    expect(percent(0.846)).toBe("85%");
    expect(percent(null)).toBe("–");
    expect(signedKes(10000)).toBe("+ KES 100.00");
    expect(signedKes(-6000)).toBe("− KES 60.00");
    expect(formatDay("2026-12-04")).toBe("4 Dec 2026");
  });

  it("says about, never a promise, for tasks left and time", () => {
    expect(tasksLeftText(undefined)).toBeNull();
    expect(tasksLeftText(0)).toBe("no tasks left for you");
    expect(tasksLeftText(1)).toBe("about 1 task left");
    expect(tasksLeftText(1200)).toBe("about 1,200 tasks left");
    expect(minutesText(undefined)).toBeNull();
    expect(minutesText(1.5)).toBe("about 1.5 min a task");
    expect(minutesText(2)).toBe("about 2 min a task");
  });
});

describe("earnings", () => {
  it("picks KES and falls back to zeros", () => {
    expect(mainTotals(summary()).approved_cents).toBe(0);
    const s = summary({
      totals: [
        {
          currency: "KES",
          pending_cents: 1,
          pending_tasks: 1,
          approved_cents: 2,
          paid_cents: 3,
          earned_cents: 5,
        },
      ],
    });
    expect(mainTotals(s).earned_cents).toBe(5);
  });

  it("explains the payout rule, with the date only when set", () => {
    expect(payoutNote(summary())).toBe(
      "Paid weekly by M-Pesa once your approved balance reaches KES 500.00.",
    );
    expect(payoutNote(summary({ next_payout_date: "2026-12-04" }))).toMatch(/^Next payout 4 Dec 2026\./);
  });

  it("names statement lines in plain words", () => {
    expect(lineLabel(line({ tasks: 4, amount_cents: 10000 }))).toBe("4 tasks accepted");
    expect(lineLabel(line({ tasks: 1, amount_cents: 2500 }))).toBe("1 task accepted");
    expect(lineLabel(line({ type: "payout", amount_cents: -6000 }))).toBe("Paid to M-Pesa");
    expect(lineLabel(line({ type: "payout_reversal", amount_cents: 6000 }))).toBe("Payment returned");
    expect(lineLabel(line({ type: "adjustment", amount_cents: -50 }))).toBe("Correction (deducted)");
  });
});

describe("trust", () => {
  it("states where the score stands", () => {
    expect(trustState(quality(null))).toBe("none");
    expect(trustState(quality({ score: 0.9, building: true }))).toBe("building");
    expect(trustState(quality({ score: 0.7 }))).toBe("good");
    expect(trustState(quality({ score: 0.69 }))).toBe("low");
    expect(trustExplanation(quality({ score: 0.9, building: true }))).toContain("20 checks");
    expect(trustExplanation(quality({ score: 0.6 }))).toContain("0.70 floor");
  });

  it("turns the trend into day offsets, skipping days without gold", () => {
    const q = quality({ score: 0.8 }, [
      { day: "2026-11-01", score: 0.8, gold_accuracy: 0.8 },
      { day: "2026-11-02", score: 0.8, gold_accuracy: null },
      { day: "2026-11-05", score: 0.9, gold_accuracy: 0.9 },
    ]);
    expect(goldTrend(q)).toEqual([
      { x: 0, y: 0.8, day: "2026-11-01" },
      { x: 4, y: 0.9, day: "2026-11-05" },
    ]);
  });
});

describe("dashboard projects", () => {
  it("keeps projects with work, at most five", () => {
    const p = (id: string, left?: number) =>
      ({ id, tasks_left: left }) as unknown as import("@/lib/api/client").WorkProject;
    const r = projectsWithWork([p("a", 0), p("b", 3), p("c"), p("d", 1), p("e", 2), p("f", 9), p("g", 4)]);
    expect(r.open).toBe(6);
    expect(r.shown.map((x) => x.id)).toEqual(["b", "c", "d", "e", "f"]);
  });
});
