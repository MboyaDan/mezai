// Pure helpers for the project screens (Stage 8). The API validates every
// field again (backend/internal/projects); these give early feedback and turn
// form text into the API's shapes.

import type { Project, ProjectInput, ProjectSkill, ProjectStatus, QAChecks } from "@/lib/api/client";
import type { TaskType } from "@/lib/onboarding";

export const taskTypes: TaskType[] = ["classification", "pairwise", "translation_review"];

export const taskTypeLabel: Record<TaskType | "other", string> = {
  classification: "Text classification",
  pairwise: "Pairwise comparison",
  translation_review: "Translation review",
  other: "Something else",
};

export const statusLabel: Record<ProjectStatus, string> = {
  draft: "Draft",
  ready: "Ready",
  live: "Live",
  paused: "Paused",
  closed: "Closed",
};

export const statusTone: Record<ProjectStatus, "neutral" | "info" | "success" | "warning" | "danger"> = {
  draft: "neutral",
  ready: "info",
  live: "success",
  paused: "warning",
  closed: "neutral",
};

export type LifecycleAction = "ready" | "draft" | "publish" | "pause" | "resume" | "close";

/** Status changes the API allows from each status (projects.actions), in button order. */
export function actionsFor(status: ProjectStatus): Array<{ action: LifecycleAction; label: string }> {
  const out: Array<{ action: LifecycleAction; label: string }> = [];
  if (status === "draft") out.push({ action: "ready", label: "Mark ready" });
  if (status === "ready")
    out.push({ action: "publish", label: "Publish" }, { action: "draft", label: "Back to draft" });
  if (status === "live") out.push({ action: "pause", label: "Pause" });
  if (status === "paused") out.push({ action: "resume", label: "Resume" });
  if (status !== "closed") out.push({ action: "close", label: "Close" });
  return out;
}

export const missingLabel: Record<string, string> = {
  instructions: "Write the contributor instructions",
  items: "Import at least one item",
  gold: "Add gold items, or set the gold rate to 0",
};

/** Readiness checklist for the next step, from the staff block. */
export function readiness(p: Project): Array<{ key: string; label: string; done: boolean }> {
  const missing = p.staff?.missing ?? [];
  const steps = [
    { key: "instructions", label: "Instructions written" },
    { key: "items", label: `Items imported (${p.item_count})` },
  ];
  if (p.gold_rate > 0) steps.push({ key: "gold", label: `Active gold items (${p.staff?.gold_count ?? 0})` });
  return steps.map((s) => ({ ...s, done: !missing.includes(s.key as never) }));
}

/** "250.50" or "250" (KES) → 25050 cents; null when not a valid amount. */
export function kesToCents(input: string): number | null {
  const s = input.trim().replace(/,/g, "");
  if (!/^\d{1,6}(\.\d{1,2})?$/.test(s)) return null;
  const [whole, frac = ""] = s.split(".");
  return Number(whole) * 100 + Number(frac.padEnd(2, "0"));
}

/** 25050 → "250.50". Integer arithmetic only. */
export function centsToKes(cents: number): string {
  const whole = Math.floor(cents / 100);
  const frac = String(cents % 100).padStart(2, "0");
  return `${whole}.${frac}`;
}

/** 25050 → "KES 250.50" with thousands separators. */
export function formatKes(cents: number, currency = "KES"): string {
  const [w, f] = centsToKes(cents).split(".");
  return `${currency} ${Number(w).toLocaleString("en-KE")}.${f}`;
}

/** "10" (%) → 0.1. Up to one decimal of a percent (the API allows three decimals of a share). */
export function percentToRate(input: string): number | null {
  const s = input.trim();
  if (!/^\d{1,2}(\.\d)?$/.test(s)) return null;
  const n = Number(s);
  return n > 50 ? null : Math.round(n * 10) / 1000;
}

export const rateToPercent = (r: number) => String(Math.round(r * 1000) / 10);

/** One entry per line (or comma), trimmed, blanks and repeats removed, order kept. */
export function listFromText(text: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of text.split(/[\n,]/)) {
    const v = raw.trim();
    if (v && !seen.has(v.toLowerCase())) {
      seen.add(v.toLowerCase());
      out.push(v);
    }
  }
  return out;
}

/** "Factual accuracy" → "factual_accuracy", the API's rubric key format. */
export function toKey(label: string): string {
  return label
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .replace(/^[^a-z]+/, "")
    .slice(0, 40);
}

export function defaultQAChecks(type: TaskType): QAChecks {
  return {
    block_unchanged_translation: type === "translation_review",
    warn_fast_seconds: 5,
    warn_rationale_mismatch: type === "pairwise",
    warn_pasted: type !== "classification",
  };
}

/** What the project form edits, as strings where people type. */
export type ProjectForm = {
  org_id: string;
  task_type: TaskType;
  name: string;
  instructions_md: string;
  labels: string;
  multi_label: boolean;
  criteria: string;
  error_tags: string;
  qa: QAChecks;
  overlap_k: string;
  gold_percent: string;
  pay_kes: string;
  target_count: string;
  deadline: string;
  min_rationale_chars: string;
  skills: ProjectSkill[];
};

export function emptyForm(type: TaskType = "classification", orgId = ""): ProjectForm {
  return {
    org_id: orgId,
    task_type: type,
    name: "",
    instructions_md: "",
    labels: "",
    multi_label: false,
    criteria: "",
    error_tags: "",
    qa: defaultQAChecks(type),
    overlap_k: "3",
    gold_percent: "10",
    pay_kes: "",
    target_count: "",
    deadline: "",
    min_rationale_chars: type === "pairwise" ? "40" : "0",
    skills: [],
  };
}

export function formFromProject(p: Project): ProjectForm {
  const t = p.task_type as TaskType;
  return {
    org_id: p.org_id,
    task_type: t,
    name: p.name,
    instructions_md: p.instructions_md,
    labels: (p.config.labels ?? []).join("\n"),
    multi_label: Boolean(p.config.multi_label),
    criteria: t === "pairwise" ? (p.config.criteria ?? []).join("\n") : "",
    error_tags: (p.config.error_tags ?? []).join("\n"),
    qa: p.staff?.qa_checks ?? defaultQAChecks(t),
    overlap_k: String(p.overlap_k),
    gold_percent: rateToPercent(p.gold_rate),
    pay_kes: p.staff ? centsToKes(p.staff.pay_per_task_cents) : "",
    target_count: p.target_count == null ? "" : String(p.target_count),
    deadline: p.deadline ?? "",
    min_rationale_chars: String(p.staff?.min_rationale_chars ?? 0),
    skills: p.skills,
  };
}

export type FormErrors = Partial<Record<string, string>>;

/** Builds the API body, or field errors keyed like the API's `field` values. */
export function toInput(f: ProjectForm, creating: boolean): { input?: ProjectInput; errors: FormErrors } {
  const errors: FormErrors = {};
  const name = f.name.trim();
  if (!name) errors.name = "Give the project a name.";
  if (creating && !f.org_id) errors.org_id = "Choose the client organisation.";

  const config: ProjectInput["config"] = {};
  if (f.task_type === "classification") {
    const labels = listFromText(f.labels);
    if (labels.length < 2) errors["config.labels"] = "List at least two labels, one per line.";
    if (labels.length > 50) errors["config.labels"] = "Use at most 50 labels.";
    config.labels = labels;
    config.multi_label = f.multi_label;
  } else if (f.task_type === "pairwise") {
    const criteria = listFromText(f.criteria).map(toKey).filter(Boolean);
    if (criteria.length < 1) errors["config.criteria"] = "List at least one rubric criterion.";
    if (criteria.length > 8) errors["config.criteria"] = "Use at most 8 criteria.";
    config.criteria = [...new Set(criteria)];
  } else {
    const tags = listFromText(f.error_tags).map(toKey).filter(Boolean);
    if (tags.length > 30) errors["config.error_tags"] = "Use at most 30 error tags.";
    if (tags.length) config.error_tags = [...new Set(tags)];
  }

  const k = Number(f.overlap_k);
  if (!Number.isInteger(k) || k < 1 || k > 5) errors.overlap_k = "Use from 1 to 5 people on each item.";
  const gold = percentToRate(f.gold_percent);
  if (gold == null) errors.gold_rate = "Use 0 to 50 percent.";
  const pay = kesToCents(f.pay_kes);
  if (pay == null) errors.pay_per_task_cents = "Enter an amount in KES, for example 12.50.";
  let target: number | null = null;
  if (f.target_count.trim()) {
    target = Number(f.target_count);
    if (!Number.isInteger(target) || target < 1)
      errors.target_count = "Use a whole number of items, or leave it empty.";
  }
  const minChars = f.task_type === "pairwise" ? Number(f.min_rationale_chars || 0) : 0;
  if (!Number.isInteger(minChars) || minChars < 0 || minChars > 2000)
    errors.min_rationale_chars = "Use 0 to 2000 characters.";
  const fast = f.qa.warn_fast_seconds;
  if (!Number.isInteger(fast) || fast < 0 || fast > 600)
    errors["qa_checks.warn_fast_seconds"] = "Use 0 to 600 seconds.";

  if (Object.keys(errors).length) return { errors };
  const qa: QAChecks = {
    block_unchanged_translation: f.task_type === "translation_review" && f.qa.block_unchanged_translation,
    warn_fast_seconds: fast,
    warn_rationale_mismatch: f.task_type === "pairwise" && f.qa.warn_rationale_mismatch,
    warn_pasted: f.qa.warn_pasted,
  };
  const input: ProjectInput = {
    name,
    instructions_md: f.instructions_md,
    config,
    qa_checks: qa,
    overlap_k: k,
    gold_rate: gold!,
    pay_per_task_cents: pay!,
    target_count: target,
    deadline: f.deadline || null,
    min_rationale_chars: minChars,
    skills: f.skills,
  };
  if (creating) {
    input.org_id = f.org_id;
    input.task_type = f.task_type;
  }
  return { input, errors };
}

/** "1 kB"-style sizes for upload reports. */
export function fileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} kB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/** Fields the import templates expect, as a CSV header line people can copy. */
export const headerLine = (cols: string[]) => cols.join(",");
