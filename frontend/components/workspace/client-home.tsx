"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { ButtonLink } from "@/components/ui/button";
import { ProgressBar } from "@/components/ui/data";
import { Alert, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { api, type Project, type ProjectProgress } from "@/lib/api/client";
import { formatDay } from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import { daysUntil, paceLabel, paceTone } from "@/lib/portal";
import { statusLabel, statusTone, taskTypeLabel } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { ActivityLog } from "./activity";
import { useCan } from "./workspace";

type Row = { project: Project; progress: ProjectProgress | null };

// Live and paused projects first, then closed, then those not started.
const order: Record<string, number> = { live: 0, paused: 1, closed: 2, ready: 3, draft: 4 };
const MAX = 8;

function deadlineText(day: string): string {
  const d = daysUntil(day);
  if (d < 0) return `Deadline was ${formatDay(day)}`;
  if (d === 0) return "Deadline today";
  return `Deadline ${formatDay(day)} (${d} ${d === 1 ? "day" : "days"})`;
}

function ProjectRow({ row }: { row: Row }) {
  const { project: p, progress: pr } = row;
  return (
    <li className="grid gap-3 px-5 py-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
        <Link href={`/w/projects/${p.id}`} className="group inline-flex items-center gap-1.5 font-medium">
          <span className="underline-offset-4 group-hover:underline">{p.name}</span>
          <ArrowRight
            aria-hidden="true"
            className="size-4 text-fg-muted transition-transform group-hover:translate-x-0.5"
          />
        </Link>
        <StatusPill status={statusTone[p.status]}>{statusLabel[p.status]}</StatusPill>
        {pr && p.status !== "draft" && p.status !== "ready" && (
          <StatusPill status={paceTone[pr.pace.state]}>{paceLabel[pr.pace.state]}</StatusPill>
        )}
      </div>
      {pr && pr.pace.goal > 0 ? (
        <ProgressBar
          label="Final items"
          value={pr.final}
          max={pr.pace.goal}
          valueText={`${pr.final.toLocaleString("en-KE")} of ${pr.pace.goal.toLocaleString("en-KE")}`}
        />
      ) : (
        <p className="text-sm text-fg-muted">Being set up with the MezAI team.</p>
      )}
      <p className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-fg-muted">
        <span>{taskTypeLabel[p.task_type as keyof typeof taskTypeLabel] ?? p.task_type}</span>
        {p.deadline && <span>{deadlineText(p.deadline)}</span>}
        {pr && pr.pace.per_day > 0 && (
          <span className="font-mono tabular">{pr.pace.per_day.toFixed(1)} a day</span>
        )}
      </p>
    </li>
  );
}

/** The client's /w: their projects with progress, and recent activity (brief 6.3). */
export function ClientHome() {
  const can = useCan();
  const rows = useLoad(async (): Promise<{ rows: Row[]; total: number }> => {
    const { projects } = await api<{ projects: Project[] }>("GET", "/v1/projects");
    const sorted = [...projects]
      .sort(
        (a, b) => (order[a.status] ?? 9) - (order[b.status] ?? 9) || b.created_at.localeCompare(a.created_at),
      )
      .slice(0, MAX);
    const progress = await Promise.all(
      sorted.map((p) =>
        p.status === "draft"
          ? Promise.resolve(null)
          : api<ProjectProgress>("GET", `/v1/projects/${p.id}/progress`).catch(() => null),
      ),
    );
    return { rows: sorted.map((project, i) => ({ project, progress: progress[i] })), total: projects.length };
  });

  return (
    <div className="grid gap-8 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
      <section aria-labelledby="projects-h" className="grid content-start gap-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 id="projects-h" className="font-display text-lg font-semibold">
            Projects
          </h2>
          {can("org.project.request") && (
            <ButtonLink href="/w/requests/new" variant="secondary" size="sm">
              Request a project
            </ButtonLink>
          )}
        </div>
        {rows.status === "loading" && <Skeleton className="h-64 w-full" />}
        {rows.status === "error" && <Alert tone="danger" title={errorMessage(rows.error)} />}
        {rows.status === "ready" &&
          (rows.data.rows.length === 0 ? (
            <EmptyState
              title="No projects yet"
              action={
                can("org.project.request") ? (
                  <ButtonLink href="/w/requests/new">Request a project</ButtonLink>
                ) : undefined
              }
            >
              Projects your organisation runs with MezAI appear here with progress and quality figures.
            </EmptyState>
          ) : (
            <>
              <ul className="divide-y divide-border rounded-card border border-border bg-surface">
                {rows.data.rows.map((r) => (
                  <ProjectRow key={r.project.id} row={r} />
                ))}
              </ul>
              {rows.data.total > rows.data.rows.length && (
                <p className="text-sm text-fg-muted">
                  Showing {rows.data.rows.length} of {rows.data.total} projects.{" "}
                  <Link href="/w/projects" className="underline underline-offset-4">
                    See all projects
                  </Link>
                </p>
              )}
            </>
          ))}
      </section>

      <section aria-labelledby="recent-h" className="grid content-start gap-3">
        <div className="flex items-center justify-between gap-3">
          <h2 id="recent-h" className="font-display text-lg font-semibold">
            Recent activity
          </h2>
          <Link href="/w/activity" className="text-sm underline underline-offset-4">
            All activity
          </Link>
        </div>
        <ActivityLog limit={6} />
      </section>
    </div>
  );
}
