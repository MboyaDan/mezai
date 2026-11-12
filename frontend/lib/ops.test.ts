import { describe, expect, it } from "vitest";
import type { OpsProject } from "@/lib/api/client";
import { atRiskSummary, dueText, queueCards, waitingText } from "./ops";

// 09:00 on 12 Nov 2026 in Nairobi.
const now = new Date("2026-11-12T06:00:00Z");

describe("waitingText", () => {
  it("counts whole Nairobi days", () => {
    expect(waitingText(null, now)).toBeNull();
    expect(waitingText("2026-11-12T05:00:00Z", now)).toBe("Oldest arrived today");
    // 22:00 UTC on 11 Nov is 01:00 on 12 Nov in Nairobi: still today.
    expect(waitingText("2026-11-11T22:00:00Z", now)).toBe("Oldest arrived today");
    expect(waitingText("2026-11-11T08:00:00Z", now)).toBe("Oldest waiting 1 day");
    expect(waitingText("2026-11-09T08:00:00Z", now)).toBe("Oldest waiting 3 days");
  });
});

describe("dueText", () => {
  it("says how far the deadline is", () => {
    expect(dueText(null, now)).toBe("No deadline");
    expect(dueText("2026-11-12", now)).toBe("Due today");
    expect(dueText("2026-11-13", now)).toBe("Due 13 Nov 2026 (1 day)");
    expect(dueText("2026-11-11", now)).toBe("1 day overdue");
    expect(dueText("2026-11-08", now)).toBe("4 days overdue");
  });
});

describe("atRiskSummary", () => {
  const p = (state: OpsProject["pace"]["state"]) => ({ pace: { state } }) as OpsProject;
  it("counts the projects at risk", () => {
    expect(atRiskSummary([p("on_track")])).toBeNull();
    expect(atRiskSummary([p("at_risk"), p("done")])).toBe(
      "1 project may miss its deadline at the pace of the last 14 days.",
    );
    expect(atRiskSummary([p("at_risk"), p("at_risk")])).toBe(
      "2 projects may miss their deadline at the pace of the last 14 days.",
    );
  });
});

describe("queueCards", () => {
  it("covers every queue once", () => {
    const keys = queueCards.map((c) => c.key);
    expect(new Set(keys).size).toBe(keys.length);
    expect(keys.sort()).toEqual(
      [
        "applications",
        "attempts",
        "identity_checks",
        "interviews",
        "project_requests",
        "reports",
        "review",
      ].sort(),
    );
  });
});
