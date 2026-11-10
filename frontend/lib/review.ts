// Pure helpers for the review queue and decisions (Stage 11a, brief 8.3).
import type { Answer } from "@/lib/onboarding";
import type { DecisionInput, ReviewDecisionKind, RouteReason } from "@/lib/api/client";

export const reasonLabel: Record<RouteReason, string> = {
  no_majority: "No majority",
  spread: "Scores far apart",
  flagged: "QA warning",
  audit_sample: "Audit sample",
};

export const reasonHelp: Record<RouteReason, string> = {
  no_majority: "The answers split with no majority, so a person decides.",
  spread: "The answers agree overall, but the scores are further apart than the project allows.",
  flagged: "An answer carries an automatic QA warning, so a person checks it.",
  audit_sample: "A random sample the project audits even when contributors agree.",
};

export const decisionLabel: Record<ReviewDecisionKind, string> = {
  accept_consensus: "Accepted the consensus",
  choose_submission: "Chose one answer",
  write_final: "Wrote the final answer",
  reject_item: "Rejected the item",
};

/** Rejection reasons and feedback need this many characters (the API checks too). */
export const MIN_REASON = 10;

export type Draft = {
  decision: ReviewDecisionKind | "";
  chosen: string;
  final: Answer | undefined;
  reason: string;
  rejections: Record<string, string>; // submission ID → reason; key present = not accepted
  feedback: Record<string, string>; // submission ID → note
};

export const emptyDraft = (): Draft => ({
  decision: "",
  chosen: "",
  final: undefined,
  reason: "",
  rejections: {},
  feedback: {},
});

export type DraftErrors = Partial<Record<string, string>>;

/** Checks a draft and builds the request body. Errors are keyed by field or submission ID. */
export function buildDecision(d: Draft): { body?: DecisionInput; errors: DraftErrors } {
  const errors: DraftErrors = {};
  if (!d.decision) errors.decision = "Choose a decision.";
  if (d.decision === "choose_submission" && !d.chosen) errors.decision = "Choose the answer to use.";
  if (d.decision === "write_final" && !d.final) errors.final = "Write the final answer.";
  if (d.decision === "reject_item" && d.reason.trim().length < MIN_REASON)
    errors.reason = `Say why the item is rejected (at least ${MIN_REASON} characters).`;
  for (const [id, text] of Object.entries(d.rejections)) {
    if (d.decision === "choose_submission" && id === d.chosen)
      errors[`reject-${id}`] = "The chosen answer cannot also be rejected.";
    else if (text.trim().length < MIN_REASON)
      errors[`reject-${id}`] = `Give the contributor a reason (at least ${MIN_REASON} characters).`;
  }
  if (Object.keys(errors).length > 0) return { errors };
  const notes = (m: Record<string, string>) =>
    Object.entries(m)
      .filter(([, t]) => t.trim())
      .map(([submission_id, t]) => ({ submission_id, text: t.trim() }));
  const body: DecisionInput = { decision: d.decision as ReviewDecisionKind };
  if (d.decision === "choose_submission") body.submission_id = d.chosen;
  if (d.decision === "write_final") body.final_answer = d.final as Record<string, unknown>;
  if (d.reason.trim()) body.reason = d.reason.trim();
  const rejections = notes(d.rejections);
  const feedback = notes(d.feedback);
  if (rejections.length) body.rejections = rejections;
  if (feedback.length) body.feedback = feedback;
  return { body, errors };
}

/** A fresh idempotency key for one decision (8 to 100 letters, digits or _.:-). */
export function decisionKey(itemId: string, rand: () => string = () => crypto.randomUUID()): string {
  return `review:${itemId}:${rand()}`.slice(0, 100);
}

/** Minutes or seconds on task, for the side-by-side view. */
export function duration(ms: number): string {
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s} s`;
  const m = Math.floor(s / 60);
  return m < 60 ? `${m} min ${s % 60} s` : `${Math.floor(m / 60)} h ${m % 60} min`;
}
