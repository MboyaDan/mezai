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
  disbursing: "Paying through M-Pesa",
  settled: "Settled",
  cancelled: "Cancelled",
};

export const batchTone: Record<BatchStatus, Tone> = {
  draft: "neutral",
  pending_approval: "warning",
  approved: "info",
  exported: "info",
  disbursing: "info",
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

export const openStatuses: BatchStatus[] = [
  "draft",
  "pending_approval",
  "approved",
  "exported",
  "disbursing",
];

/** The steps a batch goes through, with which are done; a cancelled batch has none. */
export function steps(status: BatchStatus): Array<{ label: string; state: "done" | "current" | "todo" }> {
  const order: BatchStatus[] = ["draft", "pending_approval", "approved", "exported", "settled"];
  const labels = ["Draft", "Approval", "Approved", "Paid in M-Pesa", "Settled"];
  // A B2C batch (Stage 15) is at the same step as an exported one.
  const at = order.indexOf(status === "disbursing" ? "exported" : status);
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
  | "watch" // exported or disbursing, and this person cannot act on lines
  | "b2c" // paying through M-Pesa B2C: watch results, stop a line M-Pesa never accepted
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
    case "disbursing":
      // M-Pesa B2C results arrive by themselves; Ops may stop an unsent line.
      return who.canCreate ? "b2c" : "watch";
    default:
      return "done";
  }
}

export type B2CStatus = Schemas["B2CStatus"];
export type MpesaTransaction = Schemas["MpesaTransaction"];
export type Reconciliation = Schemas["PayoutReconciliation"];
export type ReconBatch = Reconciliation["batches"][number];
export type ReconException = Reconciliation["exceptions"][number];
export type BatchTransactions = { b2c: B2CStatus; transactions: MpesaTransaction[] };

/** Where B2C payments go, in words Ops cannot misread (honesty rule 3: test modes move no money). */
export function b2cMode(b2c: B2CStatus): { label: string; real: boolean; note: string } {
  switch (b2c.environment) {
    case "production":
      return {
        label: "M-Pesa",
        real: true,
        note: "Payments go out from the ELS business account. This is real money.",
      };
    case "sandbox":
      return {
        label: "Safaricom sandbox",
        real: false,
        note: "Test mode: payments go to Safaricom's sandbox. No money moves.",
      };
    case "fake":
      return {
        label: "Test M-Pesa",
        real: false,
        note: "Test mode: a simulated M-Pesa on this server answers. No money moves.",
      };
    default:
      return {
        label: "Off",
        real: false,
        note: "Automatic M-Pesa payments are switched off. Batches are paid with the bulk payment file.",
      };
  }
}

/** One B2C payment's state for the lines table. */
export const txLabel: Record<MpesaTransaction["status"], string> = {
  queued: "Waiting to send",
  sending: "Sending",
  accepted: "Accepted",
  unknown: "Unclear",
  succeeded: "Paid",
  failed: "Failed",
};

export const txTone: Record<MpesaTransaction["status"], Tone> = {
  queued: "neutral",
  sending: "info",
  accepted: "info",
  unknown: "warning",
  succeeded: "success",
  failed: "danger",
};

/** What a payment's state means for the money, under its pill. */
export function txNote(t: Pick<MpesaTransaction, "status" | "status_queries">): string | null {
  switch (t.status) {
    case "queued":
      return "Not sent yet; can be stopped.";
    case "sending":
      return null;
    case "accepted":
      return "Waiting for M-Pesa's result.";
    case "unknown":
      return `Never sent again. ${t.status_queries === 1 ? "1 status check" : `${t.status_queries} status checks`} so far.`;
    default:
      return null;
  }
}

export const nameCheckLabel: Record<NonNullable<MpesaTransaction["name_check"]>, string> = {
  match: "Name matches",
  mismatch: "Name differs",
  unavailable: "No name returned",
};

export const exceptionLabel: Record<ReconException["kind"], string> = {
  stuck: "Waiting too long",
  late_success: "Paid after recorded failed",
  name_mismatch: "Registered name differs",
  callback: "Unexpected M-Pesa message",
};

/** True while results can still change, so the batch page refreshes itself. */
export function stillPaying(txs: MpesaTransaction[]): boolean {
  return txs.some((t) => t.status !== "succeeded" && t.status !== "failed");
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
    b2c_off: "Automatic M-Pesa payments are switched off. Export the payment file instead.",
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
