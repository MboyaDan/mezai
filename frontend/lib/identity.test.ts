import { describe, expect, it } from "vitest";
import { numberChangesPausedUntil } from "./identity";

const now = new Date("2026-11-16T12:00:00Z");
const ago = (hours: number, raised = true) => ({
  raised_by_this: raised,
  at: new Date(now.getTime() - hours * 3600_000).toISOString(),
});

describe("numberChangesPausedUntil", () => {
  it("allows changes below three tries in a day", () => {
    expect(numberChangesPausedUntil([], now)).toBeNull();
    expect(numberChangesPausedUntil([ago(1), ago(2)], now)).toBeNull();
  });

  it("ignores tries older than a day and tries made by other accounts", () => {
    expect(numberChangesPausedUntil([ago(1), ago(2), ago(25)], now)).toBeNull();
    expect(numberChangesPausedUntil([ago(1), ago(2), ago(3, false)], now)).toBeNull();
  });

  it("pauses after three tries until the oldest is a day old", () => {
    const until = numberChangesPausedUntil([ago(1), ago(5), ago(20)], now);
    expect(until?.toISOString()).toBe("2026-11-16T16:00:00.000Z");
  });

  it("with more than three, reopens when fewer than three remain", () => {
    const until = numberChangesPausedUntil([ago(1), ago(2), ago(3), ago(20)], now);
    expect(until?.toISOString()).toBe("2026-11-17T09:00:00.000Z");
  });
});
