// Ops overview helpers (Stage 13b): queue cards, waiting times and the
// deadline sentence. Pure functions, tested in ops.test.ts.

import type { OpsOverview, OpsProject } from "@/lib/api/client";
import { formatDay } from "@/lib/earnings";
import { daysUntil } from "@/lib/portal";

export type QueueKey = keyof Omit<OpsOverview["queues"], "oldest_review_at">;

/** One queue card: what it counts, where its list is, who may open it. */
export type QueueCard = { key: QueueKey; label: string; href: string; perms: string[] };

/** In the order Ops works them: client work first, then people. */
export const queueCards: QueueCard[] = [
  { key: "review", label: "Items waiting for review", href: "/w/review", perms: ["review.read"] },
  { key: "reports", label: "Content reports open", href: "/w/reports", perms: ["project.manage"] },
  {
    key: "project_requests",
    label: "Project requests to answer",
    href: "/w/requests",
    perms: ["project.manage"],
  },
  {
    key: "attempts",
    label: "Qualification attempts to decide",
    href: "/w/contributors",
    perms: ["contributor.approve"],
  },
  {
    key: "applications",
    label: "Applications to screen",
    href: "/w/contributors?tab=applications",
    perms: ["contributor.approve"],
  },
  {
    key: "interviews",
    label: "Phone interviews to hold",
    href: "/w/contributors?tab=interviews",
    perms: ["contributor.approve"],
  },
  {
    key: "identity_checks",
    label: "Identity numbers to check",
    href: "/w/contributors?tab=identity",
    perms: ["contributor.approve"],
  },
];

/** "Oldest waiting 3 days" from a timestamp, in whole Nairobi days. */
export function waitingText(since: string | null | undefined, now = new Date()): string | null {
  if (!since) return null;
  const day = new Date(new Date(since).getTime() + 3 * 3600_000).toISOString().slice(0, 10);
  const d = -daysUntil(day, now);
  if (d <= 0) return "Oldest arrived today";
  return `Oldest waiting ${d} ${d === 1 ? "day" : "days"}`;
}

/** "Due 14 Nov 2026 (2 days)", "Due today", "4 days overdue" or "No deadline". */
export function dueText(deadline: string | null, now = new Date()): string {
  if (!deadline) return "No deadline";
  const d = daysUntil(deadline, now);
  if (d < 0) return `${-d} ${d === -1 ? "day" : "days"} overdue`;
  if (d === 0) return "Due today";
  return `Due ${formatDay(deadline)} (${d} ${d === 1 ? "day" : "days"})`;
}

/** The sentence above the projects table when deadlines are at risk. */
export function atRiskSummary(projects: OpsProject[]): string | null {
  const risky = projects.filter((p) => p.pace.state === "at_risk");
  if (risky.length === 0) return null;
  return risky.length === 1
    ? "1 project may miss its deadline at the pace of the last 14 days."
    : `${risky.length} projects may miss their deadline at the pace of the last 14 days.`;
}
