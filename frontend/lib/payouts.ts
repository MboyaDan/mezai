// Payout screens (Stage 14b): labels, the batch steps, the next action for
// the person looking, and error messages. The API decides; these helpers
// only choose what to show.

import type { ApiError, Schemas } from "@/lib/api/client";
import { formatDay } from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";

export type PayoutBatch = Schemas["PayoutBatch"];
export type PayoutBatchSummary = Schemas["PayoutBatchSummary"];
export type PayoutLine = Schemas["PayoutLine"];
export type PayoutHeld = Schemas["PayoutHeld"];
export type PayoutPreview = Schemas["PayoutPreview"];
export type BatchStatus = PayoutBatchSummary["status"];
type Tone = "neutral" | "info" | "success" | "warning" | "danger";

export const batchLabel: Record<BatchStatus, string> = {
  draft: "Draft",
  pending_approval: "Waiting for approval",
  approved: "Approved",
  exported: "File exported",
  settled: "Settled",
  cancelled: "Cancelled",
};

export const batchTone: Record<BatchStatus, Tone> = {
  draft: "neutral",
  pending_approval: "warning",
  approved: "info",
  exported: "info",
  settled: "success",
  cancelled: "neutral",
};

export const lineLabel: Record<PayoutLine["status"], string> = {
  pending: "No result yet",
  paid: "Paid",
  failed: "Failed",
};

export const lineTone: Record<PayoutLine["status"], Tone> = {
  pending: "neutral",
  paid: "success",
  failed: "danger",
};

/** Why someone owed money is not in the batch, and what clears it (D-128). */
export const holdLabel: Record<PayoutHeld["reason"], string> = {
  no_phone: "No M-Pesa number on the profile",
  phone_unconfirmed: "M-Pesa number not confirmed",
  identity_unverified: "Identity not verified",
  name_unchecked: "M-Pesa registered name not checked",
  below_minimum: "Below the minimum payout",
};

export const openStatuses: BatchStatus[] = ["draft", "pending_approval", "approved", "exported"];

/** The steps a batch goes through, with which are done; a cancelled batch has none. */
export function steps(status: BatchStatus): Array<{ label: string; state: "done" | "current" | "todo" }> {
  const order: BatchStatus[] = ["draft", "pending_approval", "approved", "exported", "settled"];
  const labels = ["Draft", "Approval", "Approved", "Paid in M-Pesa", "Settled"];
  const at = order.indexOf(status);
  if (at < 0) return [];
  return labels.map((label, i) => ({
    label,
    state: i < at || status === "settled" ? "done" : i === at ? "current" : "todo",
  }));
}

export type Who = { userId: string; canCreate: boolean; canApprove: boolean };

/** What the person looking can do next with the batch. */
export type NextStep =
  | "submit" // creator side: send the draft for approval
  | "approve" // a second person approves or sends back
  | "own_batch" // waiting for someone else to approve
  | "wait_approval" // creator-side view while someone else approves
  | "export" // write the payment file
  | "wait_export" // approver view of an approved batch
  | "record" // pay in M-Pesa and record each line
  | "watch" // exported, but this person cannot record results
  | "done";

export function nextStep(b: Pick<PayoutBatch, "status" | "created_by">, who: Who): NextStep {
  switch (b.status) {
    case "draft":
      return who.canCreate ? "submit" : "done";
    case "pending_approval":
      if (who.canApprove && who.userId !== b.created_by) return "approve";
      return who.userId === b.created_by ? "own_batch" : "wait_approval";
    case "approved":
      return who.canCreate ? "export" : "wait_export";
    case "exported":
      return who.canCreate ? "record" : "watch";
    default:
      return "done";
  }
}

/** Lines whose amount is now more than the contributor's balance (approval and export refuse them). */
export function uncovered(lines: PayoutLine[]): PayoutLine[] {
  return lines.filter((l) => l.status === "pending" && l.amount_cents > l.balance_cents);
}

/** "1 Nov to 12 Nov 2026" for a batch period. */
export function periodText(start: string, end: string): string {
  return `${formatDay(start)} to ${formatDay(end)}`;
}

/** Progress of results on an exported batch. */
export function resultsText(
  b: Pick<PayoutBatchSummary, "line_count" | "paid_lines" | "failed_lines">,
): string {
  const done = b.paid_lines + b.failed_lines;
  const parts = [`${done} of ${b.line_count} recorded`];
  if (b.paid_lines) parts.push(`${b.paid_lines} paid`);
  if (b.failed_lines) parts.push(`${b.failed_lines} failed`);
  return parts.join(" · ");
}

/** Yesterday in Nairobi (UTC+3, no daylight saving): the latest period end the API accepts. */
export function latestPeriodEnd(now: Date = new Date()): string {
  const nairobi = new Date(now.getTime() + 3 * 3600_000);
  nairobi.setUTCDate(nairobi.getUTCDate() - 1);
  return nairobi.toISOString().slice(0, 10);
}

/** Today in Nairobi, the earliest next payout date. */
export function nairobiToday(now: Date = new Date()): string {
  return new Date(now.getTime() + 3 * 3600_000).toISOString().slice(0, 10);
}

/** An M-Pesa receipt as the API stores it: capitals and digits, 8 to 12 (A-077). */
export function normaliseReceipt(input: string): { receipt: string; ok: boolean } {
  const receipt = input.replace(/\s+/g, "").toUpperCase();
  return { receipt, ok: /^[A-Z0-9]{8,12}$/.test(receipt) };
}

/** A fresh idempotency key for creating one batch. */
export function batchKey(periodEnd: string, rand: () => string = () => crypto.randomUUID()): string {
  return `payout:${periodEnd}:${rand()}`.slice(0, 100);
}

/** Payout error codes in Ops' words. */
export function payoutError(e: ApiError): string {
  const m: Record<string, string> = {
    own_batch: "You created this batch, so another person with payout approval must approve it.",
    wrong_state: "The batch moved on since this page loaded. Reload to see where it is now.",
    open_batch: "Another batch is still open. Settle or cancel it before starting a new one.",
    balance_changed:
      "A contributor's balance is now less than their line, after a correction. Cancel this batch and create a new one.",
    receipt_used: "That M-Pesa receipt is already recorded on another line. Check the receipt and try again.",
    nothing_to_pay: "Nobody is due a payout for this period.",
  };
  if (m[e.code]) return m[e.code];
  if (e.code === "invalid_input" && e.field === "period_end")
    return "Choose a period end before today, after the last batch's period.";
  if (e.code === "invalid_input" && e.field === "receipt")
    return "Enter the M-Pesa receipt: 8 to 12 letters and digits, for example QK41ABCD2E.";
  if (e.code === "invalid_input" && e.field === "reason") return "Write a reason of up to 500 characters.";
  if (e.code === "invalid_input" && e.field === "date") return "Choose today or a later day.";
  return errorMessage(e);
}
