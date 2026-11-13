import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/client";
import {
  batchKey,
  latestPeriodEnd,
  nairobiToday,
  nextStep,
  normaliseReceipt,
  payoutError,
  resultsText,
  steps,
  uncovered,
  type PayoutLine,
} from "./payouts";

const ops = { userId: "u-ops", canCreate: true, canApprove: false };
const approver = { userId: "u-appr", canCreate: false, canApprove: true };
const admin = { userId: "u-admin", canCreate: true, canApprove: true };

describe("nextStep", () => {
  it("lets only a second person approve", () => {
    const b = { status: "pending_approval" as const, created_by: "u-admin" };
    expect(nextStep(b, approver)).toBe("approve");
    expect(nextStep(b, admin)).toBe("own_batch");
    expect(nextStep(b, ops)).toBe("wait_approval");
  });
  it("follows the batch through", () => {
    const by = "u-ops";
    expect(nextStep({ status: "draft", created_by: by }, ops)).toBe("submit");
    expect(nextStep({ status: "approved", created_by: by }, ops)).toBe("export");
    expect(nextStep({ status: "approved", created_by: by }, approver)).toBe("wait_export");
    expect(nextStep({ status: "exported", created_by: by }, ops)).toBe("record");
    expect(nextStep({ status: "exported", created_by: by }, approver)).toBe("watch");
    expect(nextStep({ status: "settled", created_by: by }, ops)).toBe("done");
    expect(nextStep({ status: "cancelled", created_by: by }, ops)).toBe("done");
  });
});

describe("steps", () => {
  it("marks earlier steps done and the current one", () => {
    expect(steps("approved").map((s) => s.state)).toEqual(["done", "done", "current", "todo", "todo"]);
    expect(steps("settled").every((s) => s.state === "done")).toBe(true);
    expect(steps("cancelled")).toEqual([]);
  });
});

it("finds lines no longer covered by the balance", () => {
  const line = (amount: number, balance: number, status: PayoutLine["status"] = "pending") =>
    ({ amount_cents: amount, balance_cents: balance, status }) as PayoutLine;
  expect(uncovered([line(50000, 50000), line(50000, 40000), line(50000, 0, "paid")])).toHaveLength(1);
});

it("summarises results", () => {
  expect(resultsText({ line_count: 4, paid_lines: 2, failed_lines: 1 })).toBe(
    "3 of 4 recorded · 2 paid · 1 failed",
  );
  expect(resultsText({ line_count: 2, paid_lines: 0, failed_lines: 0 })).toBe("0 of 2 recorded");
});

it("works out Nairobi days", () => {
  // 22:30 UTC on 12 Nov is 01:30 on 13 Nov in Nairobi.
  const t = new Date("2026-11-12T22:30:00Z");
  expect(nairobiToday(t)).toBe("2026-11-13");
  expect(latestPeriodEnd(t)).toBe("2026-11-12");
  expect(latestPeriodEnd(new Date("2026-03-01T06:00:00Z"))).toBe("2026-02-28");
});

it("normalises receipts", () => {
  expect(normaliseReceipt(" qk41 abcd2e ")).toEqual({ receipt: "QK41ABCD2E", ok: true });
  expect(normaliseReceipt("QK-41").ok).toBe(false);
  expect(normaliseReceipt("ABCDEFGHIJKLM").ok).toBe(false);
});

it("keeps batch keys within the API limit", () => {
  expect(batchKey("2026-11-12", () => "x".repeat(200))).toHaveLength(100);
  expect(batchKey("2026-11-12", () => "abc")).toBe("payout:2026-11-12:abc");
});

it("explains payout errors", () => {
  expect(payoutError(new ApiError(403, "own_batch"))).toMatch(/another person/);
  expect(payoutError(new ApiError(422, "invalid_input", "receipt", "x"))).toMatch(/8 to 12/);
  expect(payoutError(new ApiError(0, "network"))).toMatch(/reach the server/);
});
