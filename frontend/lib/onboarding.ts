// Pure helpers for sign-in, sign-up, profile and qualification screens.
// The API validates everything again; these give early, friendly feedback.

import { legalDocs } from "@/content/legal";
import type { ApiError, ContributorProfile, MyTest, TaskConfig } from "@/lib/api/client";

type Kind = "staff" | "client" | "contributor";

export const homeFor = (kind: Kind) => (kind === "contributor" ? "/c" : "/w");

/**
 * A return path from ?next=. Only same-site paths are allowed, so a crafted
 * link cannot send someone to another site after signing in.
 */
export function safeNext(next: string | null | undefined, kind: Kind): string {
  const home = homeFor(kind);
  if (!next || !next.startsWith("/") || next.startsWith("//") || next.includes("\\")) return home;
  // Each kind returns only into its own area.
  if (kind === "contributor" ? !next.startsWith("/c") : !next.startsWith("/w")) return home;
  return next;
}

/** Documents a contributor accepts at sign-up, as the API keys them (backend/internal/legal). */
export const requiredConsents = [
  { key: "privacy_notice", slug: "privacy" },
  { key: "contributor_terms", slug: "contributor-terms" },
] as const;

const keyToSlug: Record<string, string> = {
  privacy_notice: "privacy",
  terms_of_use: "terms",
  contributor_terms: "contributor-terms",
  acceptable_use: "acceptable-use",
};

export function legalFor(key: string) {
  const doc = legalDocs.find((d) => d.slug === keyToSlug[key]);
  if (!doc) throw new Error(`no legal document for ${key}`);
  return doc;
}

/** { privacy_notice: "0.1", ... } for the given document keys at their current versions. */
export function consentPayload(keys: readonly string[]): Record<string, string> {
  return Object.fromEntries(keys.map((k) => [k, legalFor(k).version]));
}

/** Mirrors pii.NormaliseKenyanMobile in the API. Returns null when invalid. */
export function normaliseKenyanMobile(input: string): string | null {
  let s = input.trim().replace(/[\s\-()]/g, "");
  if (s.startsWith("+254")) s = s.slice(4);
  else if (s.startsWith("254")) s = s.slice(3);
  else if (s.startsWith("0")) s = s.slice(1);
  else return null;
  return /^[71]\d{8}$/.test(s) ? `+254${s}` : null;
}

/** Mirrors identity.NormaliseNationalID in the API. Returns null when invalid. */
export function normaliseNationalId(input: string): string | null {
  const s = input.trim().replace(/[\s.-]/g, "");
  if (!/^\d+$/.test(s)) return null;
  const n = s.replace(/^0+/, "");
  return n.length >= 6 && n.length <= 9 ? n : null;
}

export const fieldLabels: Record<string, string> = {
  display_name: "Display name",
  country: "Country",
  education_level: "Education level",
  weekly_hours: "Weekly availability",
  languages: "Languages",
  domains: "Domains",
  mpesa_phone: "M-Pesa number",
  national_id: "National ID number",
  experience: "Your application answers",
};

/** The onboarding step a contributor is on, for the dashboard and redirects. */
export type Step =
  | "consent"
  | "profile"
  | "screening"
  | "declined"
  | "qualify"
  | "review"
  | "interview"
  | "identity"
  | "active"
  | "paused"
  | "removed";

/** Where the contributor is in the funnel (D-100): apply, screen, test, interview, identity. */
export function currentStep(
  p: Pick<
    ContributorProfile,
    "status" | "consent_required" | "qualifications" | "screening_status" | "interview_passed"
  >,
  tests: MyTest[],
): Step {
  if (p.status === "removed") return "removed";
  if (p.status === "paused") return "paused";
  if (p.consent_required.length > 0) return "consent";
  if (p.status === "applied") return "profile";
  if (p.status === "active") return "active";
  if (p.screening_status === "declined") return "declined";
  if (p.screening_status !== "shortlisted") return "screening";
  // The core check (D-100) is the test that counts towards activation.
  if (!p.qualifications.some((q) => q.skill_kind === "core"))
    return tests.some((t) => t.state === "awaiting_review") ? "review" : "qualify";
  // Qualified: the interview, then the identity check (D-078) remain.
  return p.interview_passed ? "identity" : "interview";
}

export const testStateLabel: Record<MyTest["state"], string> = {
  available: "Not started",
  in_progress: "In progress",
  awaiting_review: "Waiting for review",
  qualified: "Qualified",
  not_passed: "Not passed",
  expired: "Expired: take it again",
};

export type TaskType = "classification" | "pairwise" | "translation_review";
/** A qualification question's type: a project task type, or a writing sample (core check). */
export type QuestionType = TaskType | "writing";

/** Writing samples (core check): 60 to 250 words, as the API checks. */
export const writingWords = (text: string | undefined) =>
  (text ?? "").trim().split(/\s+/).filter(Boolean).length;
export const MIN_WRITING_WORDS = 60;
export const MAX_WRITING_WORDS = 250;

/** Draft answer as the screens build it; the API's Answer shape. */
export type Answer = {
  labels?: string[];
  preference?: "a" | "b" | "tie";
  scores_a?: Record<string, number>;
  scores_b?: Record<string, number>;
  rationale?: string;
  adequacy?: number;
  fluency?: number;
  corrected?: string;
  text?: string;
};

const score = (n: unknown) => typeof n === "number" && Number.isInteger(n) && n >= 1 && n <= 5;

/** Mirrors quality.Parse: true when the API will accept the answer on submit. */
export function isComplete(type: QuestionType, config: TaskConfig, a: Answer | undefined): boolean {
  if (!a) return false;
  switch (type) {
    case "classification": {
      const labels = a.labels ?? [];
      return labels.length > 0 && (config.multi_label || labels.length === 1);
    }
    case "pairwise": {
      const criteria = config.criteria ?? [];
      return (
        (a.preference === "a" || a.preference === "b" || a.preference === "tie") &&
        criteria.every((c) => score(a.scores_a?.[c]) && score(a.scores_b?.[c])) &&
        (a.rationale ?? "").trim().length > 0
      );
    }
    case "translation_review":
      return score(a.adequacy) && score(a.fluency);
    case "writing": {
      const n = writingWords(a.text);
      return n >= MIN_WRITING_WORDS && n <= MAX_WRITING_WORDS;
    }
  }
}

/** Plain-language rubric criterion: "language_quality" → "Language quality". */
export const criterionLabel = (c: string) => {
  const s = c.replace(/_/g, " ");
  return s.charAt(0).toUpperCase() + s.slice(1);
};

/** A sentence for an API error code, safe to show. */
export function errorMessage(e: ApiError): string {
  if (e.code === "invalid_input" && e.detail) return e.detail;
  if (e.code === "weak_password" && e.detail) return passwordMessage(e.detail);
  const m: Record<string, string> = {
    network: "We could not reach the server. Check your connection and try again.",
    too_many_requests: "Too many attempts. Wait a few minutes and try again.",
    invalid_credentials: "That email and password do not match an account.",
    unauthenticated: "That email and password do not match an account.",
    email_not_verified: "Confirm your email address first. We can send the link again.",
    account_not_ready: "This account is not set up yet. Contact the MezAI team.",
    consent_required: "The legal documents changed. Reload the page and accept the current versions.",
    profile_incomplete: "Complete your profile before taking a test.",
    not_eligible: "Your account is paused or removed, so tests are closed.",
    already_qualified: "You have already qualified on this test.",
    retake_too_soon: "You can retake this test once the waiting period ends.",
    test_unavailable: "This test is not open at the moment.",
    attempt_closed: "This attempt has been submitted and is waiting for review.",
    illegal_transition: "That status change is not allowed from the current status.",
    not_passed: "Only a passed attempt can be approved.",
    invalid_token: "This link is invalid or has expired. Request a new one.",
    csrf_failed: "Your session changed. Reload the page and try again.",
    forbidden: "You do not have permission to do that.",
    not_found: "We could not find that.",
    slug_taken: "Another organisation already uses that short name. Choose a different one.",
    project_closed: "This project is closed, so it cannot change.",
    not_ready: "The project is not ready for that step yet. See the checklist.",
    import_not_committable: "This file has errors or was already imported. Upload a corrected file.",
    import_stale: "The project changed after this file was checked. Check the file again.",
    file_changed: "The stored file no longer matches the checked file. Upload it again.",
    already_decided: "This request has already been decided.",
    storage_unavailable: "File storage is not responding. Try again in a minute.",
    file_too_large: "The file is too large.",
    no_gold: "This project has no active gold items left to copy.",
    no_tasks: "There are no tasks left for you on this project right now.",
    reservation_closed: "This task's time ran out, so it went back to the queue. Take the next one.",
    already_submitted: "This task was already submitted.",
    idempotency_key_reused: "Something went wrong with this submission. Reload the page and try again.",
    draft_limit: "This task has too many saved drafts. Submit or skip it.",
    project_not_live: "This project is not open for work at the moment.",
    qa_blocked: "Fix the points below before submitting.",
    already_resolved: "This report has already been resolved.",
    not_a_contributor: "Only contributors can take tasks.",
    not_shortlisted: "Tests open once the MezAI team has reviewed your application.",
    core_check_first: "Take the core check first. Skill tests open once it is approved.",
    writing_rating_required: "Rate the writing sample adequate or strong to approve, or reject with a note.",
    core_check_exists: "A core check is already set up.",
    interview_too_soon: "A new interview can be held 30 days after the last one.",
    already_passed: "This person has already passed the interview.",
    no_cv: "No CV is on file.",
    nothing_to_export: "No items are final yet, so there is nothing to export.",
    too_large: "This project has more final items than one export can hold. Contact the engineering team.",
    already_published: "This version is already published.",
    link_expired: "That download link has expired. Choose Download again.",
    identity_incomplete:
      "The contributor has not given a confirmed M-Pesa number and National ID number yet.",
    email_in_use: "That email address already has a MezAI account.",
    conflict: "That change clashes with what is already saved. Reload and check.",
  };
  return m[e.code] ?? "Something went wrong. Try again in a moment.";
}

function passwordMessage(detail: string) {
  // The API's messages start with "password: ..."; keep the sentence.
  const s = detail.replace(/^password:\s*/i, "");
  return s.charAt(0).toUpperCase() + s.slice(1) + (s.endsWith(".") ? "" : ".");
}

/** "+254 7•• ••• 678" style hint for a number being typed, or null. */
export function maskedPreview(input: string): string | null {
  const n = normaliseKenyanMobile(input);
  return n ? `+254 ${n.slice(4, 5)}•• ••• ${n.slice(10)}` : null;
}
