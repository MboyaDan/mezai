// Pure helpers for the contributor task workspace (Stage 9b). The API runs
// every check again; these give early, friendly feedback.

import type { QAIssue, Task, WorkProject } from "@/lib/api/client";
import type { Answer } from "@/lib/onboarding";

export const skipReasons = [
  { value: "unclear", label: "The item or instructions are unclear" },
  { value: "language", label: "I don't know the language well enough" },
  { value: "expertise", label: "It needs expertise I don't have" },
  { value: "technical", label: "Something is broken or won't load" },
  { value: "other", label: "Another reason" },
] as const;

export const reportCategories = [
  { value: "harmful", label: "Harmful or distressing content" },
  { value: "personal_data", label: "It contains someone's personal data" },
  { value: "broken", label: "The item is broken or empty" },
  { value: "other", label: "Something else" },
] as const;

export const reportLabel: Record<string, string> = Object.fromEntries(
  reportCategories.map((c) => [c.value, c.label]),
);

/** "29:05" (minutes:seconds) left until the reservation ends; "0:00" when past. */
export function timeLeft(expiresAt: string, now: number): string {
  const s = Math.max(0, Math.floor((Date.parse(expiresAt) - now) / 1000));
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, "0")}`;
}

export const secondsLeft = (expiresAt: string, now: number) =>
  Math.max(0, Math.floor((Date.parse(expiresAt) - now) / 1000));

/**
 * The idempotency key for one task. Kept in sessionStorage so a retry after a
 * dropped connection or a reload sends the same key (AC-07).
 */
export function submitKey(taskId: string, store: Pick<Storage, "getItem" | "setItem"> | null): string {
  const name = `mezai.submit.${taskId}`;
  try {
    const old = store?.getItem(name);
    if (old) return old;
  } catch {
    /* storage blocked: fall through to a fresh key */
  }
  const key = `t-${taskId}-${Math.random().toString(36).slice(2, 12)}`;
  try {
    store?.setItem(name, key);
  } catch {
    /* fine: the key still covers retries on this page */
  }
  return key;
}

/** Characters left before the rationale meets the project minimum, or 0. */
export function rationaleShortBy(project: Pick<WorkProject, "task_type" | "min_rationale_chars">, a: Answer) {
  if (project.task_type !== "pairwise") return 0;
  return Math.max(0, project.min_rationale_chars - [...(a.rationale ?? "").trim()].length);
}

/** The blocking issues from a `qa_blocked` API answer. */
export function blockedIssues(body: Record<string, unknown> | undefined): QAIssue[] {
  const issues = body?.issues;
  return Array.isArray(issues) ? (issues as QAIssue[]) : [];
}

/** A task the contributor can still work on. */
export const isOpen = (t: Pick<Task, "status" | "expires_at">, now: number) =>
  t.status === "reserved" && Date.parse(t.expires_at) > now;
