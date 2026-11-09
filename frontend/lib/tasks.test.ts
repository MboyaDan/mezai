import { describe, expect, it } from "vitest";
import { blockedIssues, isOpen, rationaleShortBy, submitKey, timeLeft } from "./tasks";

describe("timeLeft", () => {
  const now = Date.parse("2026-11-09T10:00:00Z");
  it("shows minutes and seconds, never negative", () => {
    expect(timeLeft("2026-11-09T10:29:05Z", now)).toBe("29:05");
    expect(timeLeft("2026-11-09T10:00:09Z", now)).toBe("0:09");
    expect(timeLeft("2026-11-09T09:59:00Z", now)).toBe("0:00");
  });
  it("treats a passed reservation as closed", () => {
    expect(isOpen({ status: "reserved", expires_at: "2026-11-09T10:01:00Z" }, now)).toBe(true);
    expect(isOpen({ status: "reserved", expires_at: "2026-11-09T09:59:00Z" }, now)).toBe(false);
    expect(isOpen({ status: "submitted", expires_at: "2026-11-09T10:30:00Z" }, now)).toBe(false);
  });
});

describe("submitKey", () => {
  it("reuses the same key for the same task and fits the API pattern", () => {
    const m = new Map<string, string>();
    const store = {
      getItem: (k: string) => m.get(k) ?? null,
      setItem: (k: string, v: string) => void m.set(k, v),
    };
    const a = submitKey("0193-abc", store);
    expect(submitKey("0193-abc", store)).toBe(a);
    expect(submitKey("0193-def", store)).not.toBe(a);
    expect(a).toMatch(/^[A-Za-z0-9_.:-]{8,100}$/);
  });
  it("works without storage", () => {
    const blocked = {
      getItem: () => {
        throw new Error("blocked");
      },
      setItem: () => {
        throw new Error("blocked");
      },
    };
    expect(submitKey("x", blocked)).toMatch(/^t-x-/);
    expect(submitKey("x", null)).toMatch(/^t-x-/);
  });
});

describe("rationaleShortBy", () => {
  const p = { task_type: "pairwise" as const, min_rationale_chars: 20 };
  it("counts characters still needed, trimmed", () => {
    expect(rationaleShortBy(p, { rationale: "  short  " })).toBe(15);
    expect(rationaleShortBy(p, { rationale: "A answers the question fully." })).toBe(0);
    expect(rationaleShortBy({ ...p, task_type: "classification" }, {})).toBe(0);
  });
});

describe("blockedIssues", () => {
  it("reads issues from the error body", () => {
    expect(blockedIssues({ error: "qa_blocked", issues: [{ code: "too_fast", message: "x" }] })).toHaveLength(
      1,
    );
    expect(blockedIssues(undefined)).toEqual([]);
  });
});
