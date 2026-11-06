import { StatusPill } from "@/components/ui/display";
import type { ContributorStatus } from "@/lib/api/client";

const map: Record<
  ContributorStatus,
  { tone: "success" | "warning" | "danger" | "neutral" | "info"; label: string }
> = {
  applied: { tone: "neutral", label: "Applied" },
  qualifying: { tone: "info", label: "Qualifying" },
  active: { tone: "success", label: "Active" },
  paused: { tone: "warning", label: "Paused" },
  removed: { tone: "danger", label: "Removed" },
};

/** Contributor status as a pill: icon and label, never colour alone. */
export function StatusBadge({ status }: { status: ContributorStatus }) {
  const s = map[status];
  return <StatusPill status={s.tone}>{s.label}</StatusPill>;
}
