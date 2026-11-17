import type { Schemas } from "@/lib/api/client";
import { formatKes } from "@/lib/projects";

type Blockers = Schemas["ErasureBlockers"];
type Retention = Schemas["Retention"];

const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

/** What must be settled before an account can be erased, in plain words (A-088). */
export function erasureWaitingOn(b: Blockers): string[] {
  const out: string[] = [];
  if (b.balance_cents > 0) out.push(`${formatKes(b.balance_cents)} approved and not yet paid`);
  if (b.balance_cents < 0) out.push(`${formatKes(-b.balance_cents)} paid in advance of approved work`);
  if (b.pending_payout_lines > 0)
    out.push(plural(b.pending_payout_lines, "payout in progress", "payouts in progress"));
  if (b.undecided_answers > 0)
    out.push(plural(b.undecided_answers, "answer waiting for a decision", "answers waiting for a decision"));
  return out;
}

const day = (iso: string) =>
  new Date(iso).toLocaleDateString("en-KE", { day: "numeric", month: "short", year: "numeric" });

/** One sentence on when a project's task content is deleted (brief 12). */
export function retentionSentence(r: Retention): string {
  if (r.purged_at) return `Task content and answers were deleted on ${day(r.purged_at)}.`;
  const after =
    r.retention_days === 0 ? "when it closes" : `${plural(r.retention_days, "day", "days")} after it closes`;
  if (r.due_at) return `Task content and answers will be deleted on ${day(r.due_at)}, ${after}.`;
  return `Task content and answers are deleted ${after}.`;
}
