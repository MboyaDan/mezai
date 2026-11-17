import { describe, expect, it } from "vitest";
import { erasureWaitingOn, retentionSentence } from "./privacy";

describe("erasureWaitingOn", () => {
  it("is empty when nothing is owed", () => {
    expect(erasureWaitingOn({ balance_cents: 0, pending_payout_lines: 0, undecided_answers: 0 })).toEqual([]);
  });
  it("names each open item with the right plural", () => {
    expect(erasureWaitingOn({ balance_cents: 5000, pending_payout_lines: 1, undecided_answers: 2 })).toEqual([
      "KES 50.00 approved and not yet paid",
      "1 payout in progress",
      "2 answers waiting for a decision",
    ]);
  });
});

describe("retentionSentence", () => {
  const base = { closed_at: null, due_at: null, purged_at: null };
  it("states the rule before the project closes", () => {
    expect(retentionSentence({ ...base, retention_days: 90 })).toBe(
      "Task content and answers are deleted 90 days after it closes.",
    );
    expect(retentionSentence({ ...base, retention_days: 0 })).toBe(
      "Task content and answers are deleted when it closes.",
    );
  });
  it("gives the date once closed, and the deletion date after", () => {
    expect(
      retentionSentence({
        ...base,
        retention_days: 1,
        closed_at: "2026-11-01T09:00:00Z",
        due_at: "2026-11-02T09:00:00Z",
      }),
    ).toBe("Task content and answers will be deleted on 2 Nov 2026, 1 day after it closes.");
    expect(retentionSentence({ ...base, retention_days: 90, purged_at: "2026-11-17T09:00:00Z" })).toBe(
      "Task content and answers were deleted on 17 Nov 2026.",
    );
  });
});
