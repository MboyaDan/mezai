// Screening funnel helpers (Stage 7d-1, D-100). The API decides; these
// mirror its rules so Ops sees the result before saving.

import type { ScreeningStatus } from "@/lib/api/client";

export const screeningLabel: Record<ScreeningStatus, string> = {
  pending: "Waiting for review",
  shortlisted: "Shortlisted",
  waitlisted: "Waiting list",
  declined: "Declined",
};

export const screeningTone: Record<ScreeningStatus, "neutral" | "info" | "success" | "warning" | "danger"> = {
  pending: "info",
  shortlisted: "success",
  waitlisted: "warning",
  declined: "danger",
};

export type InterviewForm = {
  identity_confirmed: boolean;
  confidentiality_ok: boolean;
  scenario_score: number | null;
  language_scores: Record<string, number | null>;
};

/** Mirrors contributors.InterviewPasses: identity and confidentiality, and 3+ out of 4 everywhere. */
export function interviewPasses(f: InterviewForm): boolean {
  const langs = Object.values(f.language_scores);
  return (
    f.identity_confirmed &&
    f.confidentiality_ok &&
    (f.scenario_score ?? 0) >= 3 &&
    langs.length > 0 &&
    langs.every((v) => (v ?? 0) >= 3)
  );
}

/** Every score filled in, so the call can be saved as completed. */
export function interviewComplete(f: InterviewForm): boolean {
  return f.scenario_score !== null && Object.values(f.language_scores).every((v) => v !== null);
}

/** The fixed scenario question and what each score means (D-100 section 3.4). */
export const scenario = {
  question:
    "You are halfway through a task and the instructions do not cover the case in front of you. What do you do?",
  anchors: [
    { score: 4, text: "Checks the guidelines, then skips with a reason or asks; does not guess." },
    { score: 3, text: "Would skip or ask, but does not mention the guidelines." },
    { score: 2, text: "Would make a careful guess and mention it somewhere." },
    { score: 1, text: "Would guess and carry on." },
  ],
};

export const confidentialityQuestion =
  "A friend asks to see a task you are working on. What do you tell them?";
