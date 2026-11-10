import { describe, expect, it } from "vitest";
import { buildDecision, decisionKey, duration, emptyDraft } from "./review";

describe("buildDecision", () => {
  it("needs a decision", () => {
    expect(buildDecision(emptyDraft()).errors.decision).toBeTruthy();
  });

  it("needs the chosen answer", () => {
    const r = buildDecision({ ...emptyDraft(), decision: "choose_submission" });
    expect(r.errors.decision).toMatch(/answer/);
  });

  it("builds a choice with rejections and feedback, trimmed", () => {
    const r = buildDecision({
      ...emptyDraft(),
      decision: "choose_submission",
      chosen: "s1",
      rejections: { s2: "  Ignored instruction two.  " },
      feedback: { s1: " Nice work. ", s3: "   " },
    });
    expect(r.errors).toEqual({});
    expect(r.body).toEqual({
      decision: "choose_submission",
      submission_id: "s1",
      rejections: [{ submission_id: "s2", text: "Ignored instruction two." }],
      feedback: [{ submission_id: "s1", text: "Nice work." }],
    });
  });

  it("refuses short rejection reasons and rejecting the chosen answer", () => {
    const r = buildDecision({
      ...emptyDraft(),
      decision: "choose_submission",
      chosen: "s1",
      rejections: { s1: "Long enough reason.", s2: "short" },
    });
    expect(r.body).toBeUndefined();
    expect(r.errors["reject-s1"]).toMatch(/chosen/);
    expect(r.errors["reject-s2"]).toMatch(/10/);
  });

  it("needs a reason to reject the item", () => {
    expect(
      buildDecision({ ...emptyDraft(), decision: "reject_item", reason: "bad" }).errors.reason,
    ).toBeTruthy();
    const ok = buildDecision({ ...emptyDraft(), decision: "reject_item", reason: "Not a customer message." });
    expect(ok.body).toEqual({ decision: "reject_item", reason: "Not a customer message." });
  });

  it("needs a written answer for write_final", () => {
    expect(buildDecision({ ...emptyDraft(), decision: "write_final" }).errors.final).toBeTruthy();
    const ok = buildDecision({ ...emptyDraft(), decision: "write_final", final: { labels: ["Yes"] } });
    expect(ok.body).toEqual({ decision: "write_final", final_answer: { labels: ["Yes"] } });
  });
});

describe("decisionKey", () => {
  it("matches the API's key pattern", () => {
    const k = decisionKey("0193-item", () => "abc-123");
    expect(k).toBe("review:0193-item:abc-123");
    expect(k).toMatch(/^[A-Za-z0-9_.:-]{8,100}$/);
  });
});

describe("duration", () => {
  it("formats seconds, minutes and hours", () => {
    expect(duration(4_400)).toBe("4 s");
    expect(duration(95_000)).toBe("1 min 35 s");
    expect(duration(3_700_000)).toBe("1 h 1 min");
  });
});
