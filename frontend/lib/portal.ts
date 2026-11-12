// Client portal helpers (Stage 13a): project pace, agreement figures,
// activity sentences, roles and invoice states. Pure functions, so the
// wording is tested in portal.test.ts.

import type { ActivityEntry, Invoice, ProjectProgress, ProjectQuality } from "@/lib/api/client";
import { formatDay } from "@/lib/earnings";

type Tone = "neutral" | "info" | "success" | "warning" | "danger";

/** "3 Nov": a week or day without the year, for chart labels. */
export function dayMonth(day: string): string {
  return formatDay(day).replace(/ \d{4}$/, "");
}

/** Whole days from today (Nairobi) to a YYYY-MM-DD day; negative when past. */
export function daysUntil(day: string, now = new Date()): number {
  const today = new Date(now.getTime() + 3 * 3600_000).toISOString().slice(0, 10);
  return Math.round((Date.parse(day) - Date.parse(today)) / 86_400_000);
}

/** Throughput weeks from the week the project was published; none before. */
export function weeksLive(
  weeks: ProjectProgress["throughput"],
  publishedAt: string | null | undefined,
): ProjectProgress["throughput"] {
  if (!publishedAt) return [];
  const local = new Date(new Date(publishedAt).getTime() + 3 * 3600_000);
  const monday = new Date(Date.UTC(local.getUTCFullYear(), local.getUTCMonth(), local.getUTCDate()));
  monday.setUTCDate(monday.getUTCDate() - ((monday.getUTCDay() + 6) % 7));
  const first = monday.toISOString().slice(0, 10);
  return weeks.filter((w) => w.week >= first);
}

export const paceTone: Record<ProjectProgress["pace"]["state"], Tone> = {
  done: "success",
  on_track: "success",
  at_risk: "warning",
  no_pace: "neutral",
  no_deadline: "neutral",
};

export const paceLabel: Record<ProjectProgress["pace"]["state"], string> = {
  done: "Complete",
  on_track: "On track",
  at_risk: "At risk",
  no_pace: "Not started",
  no_deadline: "No deadline",
};

/** One plain sentence about when the work will finish (A-074). */
export function paceText(p: ProjectProgress): string {
  const { pace } = p;
  const goal = `${pace.goal.toLocaleString("en-KE")} ${pace.goal === 1 ? "item" : "items"}`;
  const finish = pace.projected_finish ? formatDay(pace.projected_finish) : null;
  const deadline = p.deadline ? formatDay(p.deadline) : null;
  switch (pace.state) {
    case "done":
      return pace.goal === 1 ? "The one item is final." : `All ${goal} are final.`;
    case "on_track":
      return `At the pace of the last 14 days, the remaining ${pace.remaining.toLocaleString("en-KE")} finish by ${finish}, before the ${deadline} deadline.`;
    case "at_risk":
      return finish
        ? `At the pace of the last 14 days, the work finishes on ${finish}, after the ${deadline} deadline.`
        : `No items have been finished in the last 14 days, and the deadline is ${deadline}.`;
    case "no_pace":
      return `No items have been finished in the last 14 days. The deadline is ${deadline}.`;
    case "no_deadline":
      return finish
        ? `At the pace of the last 14 days, the remaining ${pace.remaining.toLocaleString("en-KE")} finish by ${finish}.`
        : pace.goal === 0
          ? "No items have been imported yet."
          : "No items have been finished in the last 14 days.";
  }
}

export const statisticName: Record<ProjectQuality["agreement"][number]["statistic"], string> = {
  cohen_kappa: "Cohen's κ",
  fleiss_kappa: "Fleiss' κ",
  percent: "Percent agreement",
  mean_abs_diff: "Mean difference",
};

type Figure = ProjectQuality["agreement"][number];
const isKappa = (f: Figure) => f.statistic === "cohen_kappa" || f.statistic === "fleiss_kappa";

/** The project-wide kappa and percent agreement (either may be missing). */
export function projectAgreement(q: ProjectQuality): { kappa?: Figure; percent?: Figure; spread?: Figure } {
  const whole = q.agreement.filter((f) => f.label == null);
  return {
    kappa: whole.find(isKappa),
    percent: whole.find((f) => f.statistic === "percent"),
    spread: whole.find((f) => f.statistic === "mean_abs_diff"),
  };
}

/** One figure per label, kappa when there is one, otherwise percent. */
export function labelAgreement(q: ProjectQuality): Figure[] {
  const byLabel = new Map<string, Figure>();
  for (const f of q.agreement) {
    if (f.label == null || !(isKappa(f) || f.statistic === "percent")) continue;
    const cur = byLabel.get(f.label);
    if (!cur || (isKappa(f) && !isKappa(cur))) byLabel.set(f.label, f);
  }
  return [...byLabel.values()].sort((a, b) => b.value - a.value || a.label!.localeCompare(b.label!));
}

export const roleLabel: Record<string, string> = {
  client_admin: "Client Admin",
  client_viewer: "Client Viewer",
};

export const invoiceTone: Record<Invoice["status"], Tone> = {
  issued: "info",
  paid: "success",
  void: "neutral",
};
export const invoiceLabel: Record<Invoice["status"], string> = {
  issued: "Issued",
  paid: "Paid",
  void: "Void",
};

const projectStatusVerb: Record<string, string> = {
  draft: "moved back to draft",
  ready: "marked ready",
  live: "published",
  paused: "paused",
  closed: "closed",
};

/**
 * The activity entry as a sentence without the actor, e.g. "published
 * version 2 of the deliverables". The project, when there is one, is shown
 * separately as a link.
 */
export function activityText(a: ActivityEntry): string {
  const rows = Number(a.rows || 0);
  switch (a.action) {
    case "project.created":
      return "created the project";
    case "project.updated":
      return "updated the project set-up";
    case "project.status_changed":
      return projectStatusVerb[a.status ?? ""] ?? "changed the project status";
    case "import.committed":
      return `added ${rows.toLocaleString("en-KE")} ${rows === 1 ? "item" : "items"} from a file`;
    case "deliverable.published":
      return `published deliverable version ${a.version}`;
    case "deliverable.downloaded":
      return `downloaded the ${a.file === "manifest" ? "manifest" : "data file"} of version ${a.version}`;
    case "project_request.created":
      return `sent the project request “${a.subject}”`;
    case "project_request.converted":
      return `turned the request “${a.subject}” into a project`;
    case "project_request.declined":
      return `declined the request “${a.subject}”`;
    case "invitation.created":
      return `invited ${a.subject}`;
    case "invitation.accepted":
      return "accepted an invitation and joined";
    case "invitation.revoked":
      return `cancelled the invitation for ${a.subject}`;
    case "user.role_granted":
      return `gave ${a.subject} a role`;
    case "user.role_removed":
      return `removed a role from ${a.subject}`;
    case "invoice.uploaded":
      return `added invoice ${a.subject}`;
    case "invoice.paid":
      return `marked invoice ${a.subject} paid`;
    case "invoice.voided":
      return `voided invoice ${a.subject}`;
    case "invoice.downloaded":
      return `downloaded invoice ${a.subject}`;
    default:
      return a.action;
  }
}
