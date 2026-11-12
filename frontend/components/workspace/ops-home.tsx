"use client";

import Link from "next/link";
import { useState } from "react";
import { ArrowRight } from "lucide-react";
import { Stat, StatGroup } from "@/components/ui/data";
import { Button } from "@/components/ui/button";
import { Alert, Card, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { Table, type Column } from "@/components/ui/table";
import { api, type OpsOverview, type OpsProject } from "@/lib/api/client";
import { percent, score } from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import { atRiskSummary, dueText, queueCards, waitingText } from "@/lib/ops";
import { paceLabel, paceTone } from "@/lib/portal";
import { statusLabel, statusTone, taskTypeLabel } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { useCan } from "./workspace";

/** At-risk projects come first, so the first rows are the ones to act on. */
const PROJECTS_SHOWN = 10;

const n = (v: number) => v.toLocaleString("en-KE");

function QueueCount({
  label,
  value,
  href,
  caption,
}: {
  label: string;
  value: number;
  href: string;
  caption?: string | null;
}) {
  return (
    <Link href={href} className="group">
      <Card interactive className="grid h-full content-between gap-2 p-4">
        <span className="text-sm text-fg-muted">{label}</span>
        <span className="flex items-end justify-between gap-2">
          <span className={`font-mono text-3xl font-medium tabular ${value === 0 ? "text-fg-muted" : ""}`}>
            {n(value)}
          </span>
          <ArrowRight
            aria-hidden="true"
            className="mb-1.5 size-4 text-fg-muted transition-transform group-hover:translate-x-1"
          />
        </span>
        {caption && <span className="text-xs text-fg-muted">{caption}</span>}
      </Card>
    </Link>
  );
}

const projectColumns: Column<OpsProject>[] = [
  {
    key: "name",
    header: "Project",
    sortValue: (p) => p.name,
    cell: (p) => (
      <span className="grid min-w-44 gap-0.5">
        <Link href={`/w/projects/${p.id}`} className="font-medium underline-offset-4 hover:underline">
          {p.name}
        </Link>
        <span className="text-xs text-fg-muted">
          {p.org_name} · {taskTypeLabel[p.task_type as keyof typeof taskTypeLabel] ?? p.task_type}
        </span>
      </span>
    ),
  },
  {
    key: "status",
    header: "Status",
    cell: (p) => (
      <span className="flex flex-wrap gap-1.5 whitespace-nowrap">
        <StatusPill status={statusTone[p.status]}>{statusLabel[p.status]}</StatusPill>
        {(p.status === "live" || p.status === "paused") && p.pace.state !== "no_deadline" && (
          <StatusPill status={paceTone[p.pace.state]}>{paceLabel[p.pace.state]}</StatusPill>
        )}
      </span>
    ),
  },
  {
    key: "deadline",
    header: "Deadline",
    sortValue: (p) => p.deadline ?? "9999",
    cell: (p) => <span className="whitespace-nowrap">{dueText(p.deadline)}</span>,
  },
  {
    key: "progress",
    header: "Final items",
    sortValue: (p) => (p.pace.goal ? p.final / p.pace.goal : 0),
    cell: (p) =>
      p.pace.goal > 0 ? (
        <span className="grid min-w-40 gap-1.5">
          <span className="flex justify-between gap-3 text-sm whitespace-nowrap">
            <span className="font-mono tabular">
              {n(p.final)} of {n(p.pace.goal)}
            </span>
            <span className="font-mono text-fg-muted tabular">{p.pace.per_day.toFixed(1)} a day</span>
          </span>
          <span
            role="progressbar"
            aria-label={`Final items, ${p.name}`}
            aria-valuemin={0}
            aria-valuemax={p.pace.goal}
            aria-valuenow={p.final}
            aria-valuetext={`${n(p.final)} of ${n(p.pace.goal)}`}
            className="block h-1.5 overflow-hidden rounded-full bg-border"
          >
            <span
              className="block h-full rounded-full bg-accent-fg"
              style={{ width: `${Math.min(p.final / p.pace.goal, 1) * 100}%` }}
            />
          </span>
        </span>
      ) : (
        <span className="text-fg-muted">No items yet</span>
      ),
  },
  {
    key: "review",
    header: "In review",
    numeric: true,
    sortValue: (p) => p.in_review,
    cell: (p) => n(p.in_review),
  },
  {
    key: "active",
    header: "Active",
    numeric: true,
    sortValue: (p) => p.active_contributors,
    cell: (p) => n(p.active_contributors),
  },
  {
    key: "gold",
    header: "Gold",
    numeric: true,
    sortValue: (p) => p.gold_accuracy ?? -1,
    cell: (p) => percent(p.gold_accuracy),
  },
];

type Watch = OpsOverview["contributors"]["watch"][number];

function watchColumns(floor: number): Column<Watch>[] {
  return [
    {
      key: "who",
      header: "Contributor",
      cell: (w) => (
        <span className="grid gap-0.5">
          <Link href={`/w/contributors/${w.id}`} className="font-medium underline underline-offset-4">
            {w.name}
          </Link>
          <span className="font-mono text-xs text-fg-muted">{w.public_code}</span>
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      cell: (w) =>
        w.status === "paused" ? (
          <StatusPill status="warning">{w.auto_paused ? "Paused automatically" : "Paused"}</StatusPill>
        ) : w.score < floor ? (
          <StatusPill status="danger">Below the floor</StatusPill>
        ) : (
          <StatusPill status="success">Active</StatusPill>
        ),
    },
    { key: "score", header: "Trust score", numeric: true, cell: (w) => score(w.score) },
    { key: "gold", header: "Gold accuracy", numeric: true, cell: (w) => percent(w.gold_accuracy) },
  ];
}

/** The Ops home page (Stage 13b, brief 6.3): queues, projects, contributor health. */
export function OpsHome() {
  const can = useCan();
  const o = useLoad(() => api<OpsOverview>("GET", "/v1/ops/overview"));
  const [all, setAll] = useState(false);

  if (o.status === "loading") return <Skeleton className="h-96 w-full" />;
  if (o.status === "error") return <Alert tone="danger" title={errorMessage(o.error)} />;
  const { queues, projects, contributors: c } = o.data;
  const cards = queueCards.filter((card) => can(...card.perms));
  const risk = atRiskSummary(projects);

  return (
    <div className="grid gap-10">
      <section aria-labelledby="queues-h" className="grid gap-3">
        <h2 id="queues-h" className="font-display text-lg font-semibold">
          Waiting for the team
        </h2>
        <ul className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
          {cards.map((card) => (
            <li key={card.key} className="grid">
              <QueueCount
                label={card.label}
                value={queues[card.key]}
                href={card.href}
                caption={card.key === "review" ? waitingText(queues.oldest_review_at) : null}
              />
            </li>
          ))}
        </ul>
      </section>

      <section aria-labelledby="ops-projects-h" className="grid gap-3">
        <div className="flex flex-wrap items-baseline justify-between gap-3">
          <h2 id="ops-projects-h" className="font-display text-lg font-semibold">
            Open projects
          </h2>
          <Link href="/w/projects" className="text-sm underline underline-offset-4">
            All projects
          </Link>
        </div>
        {risk && (
          <Alert tone="warning" title={risk}>
            At-risk projects are listed first. The pace counts items made final in the last 14 days; a project
            with none and a deadline within 7 days is at risk too.
          </Alert>
        )}
        {projects.length === 0 ? (
          <EmptyState title="No open projects">
            Projects that are not closed appear here with their pace against the deadline.
          </EmptyState>
        ) : (
          <>
            <Table
              caption="Open projects, at-risk first, then by deadline"
              hideCaption
              columns={projectColumns}
              rows={all ? projects : projects.slice(0, PROJECTS_SHOWN)}
              rowKey={(p) => p.id}
              empty="No open projects."
            />
            <p className="text-xs text-fg-muted">
              Active: contributors who submitted in the last 7 days. Gold: accuracy on gold checks in the last
              30 days.
            </p>
            {projects.length > PROJECTS_SHOWN && (
              <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-fg-muted">
                <span>
                  Showing {all ? n(projects.length) : PROJECTS_SHOWN} of {n(projects.length)} open projects.
                </span>
                <Button variant="ghost" size="sm" onClick={() => setAll((v) => !v)} aria-expanded={all}>
                  {all ? "Show fewer" : `Show all ${n(projects.length)}`}
                </Button>
              </div>
            )}
          </>
        )}
      </section>

      <section aria-labelledby="health-h" className="grid gap-3">
        <div className="grid gap-1">
          <h2 id="health-h" className="font-display text-lg font-semibold">
            Contributor health
          </h2>
          <p className="text-sm text-fg-muted">
            Trust scores count once a contributor has {n(c.min_gold_results)} gold checks. Below{" "}
            {score(c.floor)} a contributor is paused automatically at the next recalculation.
          </p>
        </div>
        <Card className="p-5">
          <StatGroup>
            <Stat label="Active" value={n(c.active)} caption={`${n(c.building)} still building a score`} />
            <Stat label="Worked in the last 7 days" value={n(c.worked_last_7_days)} />
            <Stat
              label="Paused"
              value={n(c.paused)}
              caption={`${n(c.auto_paused_last_30_days)} automatically in 30 days`}
            />
            <Stat
              label="Active below the floor"
              value={n(c.active_below_floor)}
              caption="Paused at the next recalculation"
            />
            <Stat
              label="Applicants"
              value={n(c.applied + c.qualifying)}
              caption={`${n(c.applied)} applied, ${n(c.qualifying)} qualifying`}
            />
          </StatGroup>
        </Card>
        {c.watch.length > 0 ? (
          <Table
            caption="Lowest trust scores"
            columns={watchColumns(c.floor)}
            rows={c.watch}
            rowKey={(w) => w.id}
            empty="No settled trust scores yet."
          />
        ) : (
          <p className="text-sm text-fg-muted">
            No contributor has a settled trust score yet, so there is no low-score list.
          </p>
        )}
      </section>
    </div>
  );
}
