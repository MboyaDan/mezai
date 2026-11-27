// API paths a workspace page reads on the server for its first HTML, shared
// by the page (server) and the component (browser) so both use the same key
// (Stage 27, D-169). Only plain values here: no "use client" module.

export const ATTEMPTS_PATH = "/v1/qualification/attempts";
export const CONTRIBUTORS_PATH = "/v1/contributors";

const contributorTabs = ["all", "identity", "applications", "interviews", "erasure"];

/** The tab /w/contributors opens on. People who cannot approve see everyone. */
export function contributorsTab(canApprove: boolean, tab: string | null | undefined): string {
  if (!canApprove) return "all";
  return tab && contributorTabs.includes(tab) ? tab : "queue";
}

/** The path the opening tab reads first, if the server should read it. */
export function contributorsTabPath(tab: string): `/v1/${string}` | null {
  if (tab === "queue") return ATTEMPTS_PATH;
  if (tab === "all") return CONTRIBUTORS_PATH;
  return null;
}

/** The review queue's path for the page's filters. */
export function queuePath(project: string, reason: string) {
  const q = new URLSearchParams();
  if (project) q.set("project_id", project);
  if (reason) q.set("reason", reason);
  const s = q.toString();
  return `/v1/review/items${s ? `?${s}` : ""}` as const;
}
