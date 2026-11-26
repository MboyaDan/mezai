// The Ops enquiries inbox (Stage 24, brief 6.3 and 9). States and allowed
// moves come from the API (`moves`); this file only names them.
import type { Schemas } from "@/lib/api/client";
import { projectTypes, timelines, volumes, type Option } from "@/content/contact";

export type EnquiryStatus = Schemas["EnquiryStatus"];
export type EnquirySummary = Schemas["EnquirySummary"];
export type EnquiryDetail = Schemas["EnquiryDetail"];
export type EnquiryNote = Schemas["EnquiryNote"];

export const enquiryStatuses: EnquiryStatus[] = ["new", "assigned", "in_conversation", "won", "lost", "spam"];
export const openStatuses: EnquiryStatus[] = ["new", "assigned", "in_conversation"];

export const statusText: Record<
  EnquiryStatus,
  { label: string; tone: "info" | "warning" | "success" | "neutral" | "danger" }
> = {
  new: { label: "New", tone: "warning" },
  assigned: { label: "Assigned", tone: "info" },
  in_conversation: { label: "In conversation", tone: "info" },
  won: { label: "Won", tone: "success" },
  lost: { label: "Lost", tone: "neutral" },
  spam: { label: "Spam", tone: "neutral" },
};

/** The button for moving from `from` to `to`. */
export function moveLabel(from: EnquiryStatus, to: EnquiryStatus): string {
  if (to === "new") return from === "spam" ? "Not spam" : "Unassign";
  if (to === "in_conversation") return from === "lost" ? "Reopen" : "Start conversation";
  if (to === "won") return "Mark won";
  if (to === "lost") return "Mark lost";
  if (to === "spam") return "Mark as spam";
  return "Assign";
}

/** Assigning is done with the person picker, so "assigned" has no button. */
export const buttonMoves = (moves: EnquiryStatus[]) => moves.filter((m) => m !== "assigned");

/** Moves that need a person: the API refuses them without one. */
export const needsAssignee = (s: EnquiryStatus) => s === "assigned" || s === "in_conversation";

const labelIn = (list: Option[], key: string) =>
  list.find((o) => o.value === key)?.label ?? (key || "Not given");
export const projectTypeLabel = (k: string) => labelIn(projectTypes, k);
export const volumeLabel = (k: string) => labelIn(volumes, k);
export const timelineLabel = (k: string) => labelIn(timelines, k);

/** Builds the PATCH body: the whole wanted state plus the status the caller saw. */
export function change(
  e: Pick<EnquiryDetail, "status" | "assigned_to" | "org_id">,
  to: Partial<{ status: EnquiryStatus; assigned_to: string | null; org_id: string | null }>,
  me: string,
) {
  let status = to.status ?? e.status;
  let assigned_to = to.assigned_to !== undefined ? to.assigned_to : e.assigned_to;
  if (to.status === "new") {
    assigned_to = null; // unassign, or back from spam
  } else if (status === "new" && assigned_to) {
    status = "assigned"; // picking someone assigns it
  } else if (status === "assigned" && !assigned_to) {
    status = "new"; // picking nobody unassigns it
  }
  // Reopening a lost enquiry, or starting with nobody assigned, takes it on.
  if (needsAssignee(status) && !assigned_to) assigned_to = me;
  const org_id = status === "won" ? (to.org_id !== undefined ? to.org_id : e.org_id) : null;
  return { expected_status: e.status, status, assigned_to, org_id };
}
