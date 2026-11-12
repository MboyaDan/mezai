import { describe, expect, it } from "vitest";
import type { ActivityEntry, ProjectProgress, ProjectQuality } from "@/lib/api/client";
import {
  activityText,
  dayMonth,
  daysUntil,
  labelAgreement,
  paceText,
  projectAgreement,
  weeksLive,
} from "./portal";

const progress = (
  pace: Partial<ProjectProgress["pace"]>,
  deadline: string | null = null,
): ProjectProgress => ({
  project_id: "p",
  total: 100,
  not_started: 0,
  in_progress: 0,
  in_review: 0,
  final: 80,
  rejected: 0,
  target: null,
  deadline,
  throughput: [],
  pace: { goal: 100, remaining: 20, per_day: 2, projected_finish: null, state: "no_deadline", ...pace },
});

describe("paceText", () => {
  it("says when the work finishes against the deadline", () => {
    expect(paceText(progress({ state: "on_track", projected_finish: "2026-11-21" }, "2026-11-30"))).toBe(
      "At the pace of the last 14 days, the remaining 20 finish by 21 Nov 2026, before the 30 Nov 2026 deadline.",
    );
    expect(paceText(progress({ state: "at_risk", projected_finish: "2026-12-02" }, "2026-11-30"))).toBe(
      "At the pace of the last 14 days, the work finishes on 2 Dec 2026, after the 30 Nov 2026 deadline.",
    );
    expect(paceText(progress({ state: "at_risk" }, "2026-11-14"))).toBe(
      "No items have been finished in the last 14 days, and the deadline is 14 Nov 2026.",
    );
    expect(paceText(progress({ state: "done", goal: 1, remaining: 0 }))).toBe("The one item is final.");
    expect(paceText(progress({ state: "done", goal: 40, remaining: 0 }))).toBe("All 40 items are final.");
    expect(paceText(progress({ state: "no_deadline", goal: 0 }))).toBe("No items have been imported yet.");
  });
});

describe("weeksLive", () => {
  const weeks = ["2026-10-26", "2026-11-02", "2026-11-09"].map((week) => ({ week, final: 1 }));
  it("starts at the Nairobi week the project was published", () => {
    // Sunday 8 Nov 22:30 UTC is Monday 9 Nov in Nairobi.
    expect(weeksLive(weeks, "2026-11-08T22:30:00Z").map((w) => w.week)).toEqual(["2026-11-09"]);
    expect(weeksLive(weeks, "2026-11-04T10:00:00Z").map((w) => w.week)).toEqual(["2026-11-02", "2026-11-09"]);
    expect(weeksLive(weeks, null)).toEqual([]);
  });
});

describe("dates", () => {
  it("drops the year for chart labels", () => {
    expect(dayMonth("2026-11-03")).toBe("3 Nov");
  });
  it("counts days in Nairobi time", () => {
    // 22:00 UTC on 10 Nov is 01:00 on 11 Nov in Nairobi.
    expect(daysUntil("2026-11-12", new Date("2026-11-10T22:00:00Z"))).toBe(1);
    expect(daysUntil("2026-11-10", new Date("2026-11-10T22:00:00Z"))).toBe(-1);
  });
});

const fig = (
  label: string | null,
  statistic: ProjectQuality["agreement"][number]["statistic"],
  value: number,
): ProjectQuality["agreement"][number] => ({
  label,
  statistic,
  value,
  band: null,
  items: 10,
  raters: 3,
  window_end: "2026-11-10T00:00:00Z",
});

describe("agreement figures", () => {
  const q = {
    agreement: [
      fig(null, "percent", 0.82),
      fig(null, "fleiss_kappa", 0.61),
      fig("Billing", "percent", 0.9),
      fig("Billing", "fleiss_kappa", 0.71),
      fig("Other", "percent", 0.5),
    ],
  } as ProjectQuality;
  it("finds the whole-project kappa and percent", () => {
    const a = projectAgreement(q);
    expect(a.kappa?.value).toBe(0.61);
    expect(a.percent?.value).toBe(0.82);
  });
  it("keeps one figure per label, preferring kappa", () => {
    expect(labelAgreement(q).map((f) => [f.label, f.statistic])).toEqual([
      ["Billing", "fleiss_kappa"],
      ["Other", "percent"],
    ]);
  });
});

describe("activityText", () => {
  const entry = (action: string, extra: Partial<ActivityEntry> = {}): ActivityEntry => ({
    id: "1",
    at: "2026-11-10T09:00:00Z",
    action,
    actor: { kind: "mezai", name: "MezAI team" },
    project_id: null,
    project_name: null,
    subject: "",
    ...extra,
  });
  it("writes a sentence for each client action", () => {
    expect(activityText(entry("project.status_changed", { status: "live" }))).toBe("published");
    expect(activityText(entry("import.committed", { rows: "1" }))).toBe("added 1 item from a file");
    expect(activityText(entry("deliverable.downloaded", { file: "data", version: "2" }))).toBe(
      "downloaded the data file of version 2",
    );
    expect(activityText(entry("invitation.created", { subject: "a@sample.invalid" }))).toBe(
      "invited a@sample.invalid",
    );
    expect(activityText(entry("invoice.voided", { subject: "MZ-1" }))).toBe("voided invoice MZ-1");
  });
});
