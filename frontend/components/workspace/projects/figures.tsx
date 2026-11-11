"use client";

import { useState } from "react";
import { RefreshCw } from "lucide-react";
import { BarChart } from "@/components/charts/bar-chart";
import { Button } from "@/components/ui/button";
import { Agreement, ProgressBar, Stat, StatGroup } from "@/components/ui/data";
import { Alert, Card, Skeleton, StatusPill, Tag } from "@/components/ui/display";
import { Table, type Column } from "@/components/ui/table";
import { labelFor } from "@/content/profile";
import { api, type ProjectProgress, type ProjectQuality, type Workforce } from "@/lib/api/client";
import { percent } from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import {
  dayMonth,
  labelAgreement,
  weeksLive,
  paceLabel,
  paceText,
  paceTone,
  projectAgreement,
  statisticName,
} from "@/lib/portal";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "../contributors";
import { useCan } from "../workspace";
import { useProject } from "./frame";

const n = (v: number) => v.toLocaleString("en-KE");

/**
 * The project's progress and quality (DESIGN.md section 11, wireframe 5):
 * a KPI row, the pace against the deadline, items by stage, and two charts.
 * Ops and the client see the same figures; gold checks are never counted.
 */
export function ProjectFigures() {
  const { project: p } = useProject();
  const can = useCan();
  const progress = useLoad(() => api<ProjectProgress>("GET", `/v1/projects/${p.id}/progress`), p.id);
  const [refresh, setRefresh] = useState(0);
  const quality = useLoad(
    () => api<ProjectQuality>("GET", `/v1/projects/${p.id}/quality`),
    `${p.id}/${refresh}`,
  );
  const [busy, setBusy] = useState(false);

  if (progress.status === "loading" || quality.status === "loading")
    return <Skeleton className="h-64 w-full" />;
  if (progress.status === "error") return <Alert tone="danger" title={errorMessage(progress.error)} />;
  if (quality.status === "error") return <Alert tone="danger" title={errorMessage(quality.error)} />;
  const pr = progress.data;
  const q = quality.data;

  if (pr.total === 0) {
    return (
      <Alert tone="info" title="Progress and quality figures start when items are added">
        Completed items, pace, gold accuracy and agreement appear here once the project has items and
        contributors begin work.
      </Alert>
    );
  }

  const snapshot = async () => {
    setBusy(true);
    try {
      await api("POST", `/v1/projects/${p.id}/quality/snapshot`);
      setRefresh((r) => r + 1);
    } finally {
      setBusy(false);
    }
  };

  const agreement = projectAgreement(q);
  const weeks = weeksLive(pr.throughput, p.published_at);
  const labels = labelAgreement(q);
  const stages: Array<[string, number]> = [
    ["Not started", pr.not_started],
    ["Being answered", pr.in_progress],
    ["In review", pr.in_review],
    ["Final", pr.final],
    ["Rejected", pr.rejected],
  ];

  return (
    <section aria-labelledby="figures-h" className="grid gap-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="figures-h" className="font-display text-lg font-semibold">
          Progress and quality
        </h2>
        {can("project.manage") && (
          <Button variant="ghost" size="sm" onClick={snapshot} disabled={busy}>
            <RefreshCw aria-hidden="true" className="size-4" />
            {busy ? "Recalculating…" : "Recalculate agreement"}
          </Button>
        )}
      </div>

      <Card className="grid gap-6 p-5 md:p-6">
        <StatGroup>
          <Stat
            label="Final items"
            value={n(pr.final)}
            unit={`of ${n(pr.pace.goal)}`}
            caption={pr.target ? "Against the target" : "Against items imported"}
            className="col-span-2 sm:col-span-1"
          />
          <Stat label="Pace" value={pr.pace.per_day.toFixed(1)} unit="a day" caption="Last 14 days" />
          <Stat
            label="Gold accuracy"
            value={percent(q.gold_accuracy)}
            caption={`${n(q.gold_results)} checks, 30 days`}
          />
          <Stat
            label="Agreement"
            value={
              agreement.kappa ? (
                <Agreement value={agreement.kappa.value} className="text-[1.75rem] leading-tight" />
              ) : agreement.percent ? (
                percent(agreement.percent.value)
              ) : (
                "–"
              )
            }
            caption={
              agreement.kappa
                ? `${statisticName[agreement.kappa.statistic]}${agreement.percent ? ` · ${percent(agreement.percent.value)} agree` : ""}`
                : agreement.percent
                  ? "Percent agreement"
                  : p.overlap_k === 1
                    ? "One answer per item"
                    : "Not measured yet"
            }
          />
          <Stat
            label="Review rate"
            value={percent(q.review_rate)}
            caption={`${n(q.routed_to_review)} of ${n(q.completed)} completed`}
          />
        </StatGroup>

        <ProgressBar
          label="Final items"
          value={pr.final}
          max={Math.max(pr.pace.goal, 1)}
          valueText={`${n(pr.final)} of ${n(pr.pace.goal)} (${percent(pr.pace.goal ? pr.final / pr.pace.goal : 0)})`}
        />

        <div className="flex flex-wrap items-start gap-x-3 gap-y-2 border-t border-border pt-5">
          <StatusPill status={paceTone[pr.pace.state]}>{paceLabel[pr.pace.state]}</StatusPill>
          <p className="min-w-0 flex-1 text-sm text-fg-muted">{paceText(pr)}</p>
        </div>

        <dl className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm sm:grid-cols-5">
          {stages.map(([k, v]) => (
            <div key={k} className="grid gap-0.5">
              <dt className="text-fg-muted">{k}</dt>
              <dd className="font-mono tabular">{n(v)}</dd>
            </div>
          ))}
        </dl>
      </Card>

      {weeks.length > 0 && (
        <div className={labels.length > 0 ? "grid gap-6 lg:grid-cols-2" : "grid gap-6"}>
          <Card className="p-5">
            <BarChart
              title="Items made final per week"
              note="Weeks start on Monday, Nairobi time."
              data={weeks.map((w) => ({ label: `Week of ${dayMonth(w.week)}`, value: w.final }))}
              format={n}
            />
          </Card>
          {labels.length > 0 && (
            <Card className="p-5">
              <BarChart
                title="Agreement by label"
                note={`${statisticName[labels[0].statistic]} where there is one, otherwise percent agreement. Last 30 days.`}
                data={labels.map((f) => ({ label: f.label!, value: f.value }))}
                max={1}
                format={(v) => v.toFixed(2)}
                series={2}
              />
            </Card>
          )}
        </div>
      )}
    </section>
  );
}

const workerColumns: Column<Workforce["workers"][number]>[] = [
  { key: "code", header: "Contributor", cell: (w) => <span className="font-mono">{w.code}</span> },
  { key: "tasks", header: "Answers", numeric: true, cell: (w) => n(w.tasks), sortValue: (w) => w.tasks },
  {
    key: "last",
    header: "Last answer",
    cell: (w) => shortDate(w.last_submitted_at),
    sortValue: (w) => w.last_submitted_at,
  },
];

/** Who works on the project: counts, skills and per-client codes only (brief 6.3, D-123). */
export function WorkforcePanel() {
  const { project: p } = useProject();
  const wf = useLoad(() => api<Workforce>("GET", `/v1/projects/${p.id}/workforce`), p.id);
  if (wf.status === "loading") return <Skeleton className="h-40 w-full" />;
  if (wf.status === "error") return <Alert tone="danger" title={errorMessage(wf.error)} />;
  const w = wf.data;
  const langs = w.skills.filter((s) => s.kind === "language");
  const domains = w.skills.filter((s) => s.kind === "domain");
  return (
    <section aria-labelledby="workforce-h" className="grid gap-3">
      <div className="grid gap-1">
        <h2 id="workforce-h" className="font-display font-semibold">
          Workforce
        </h2>
        <p className="text-sm text-fg-muted">
          {p.staff
            ? "The codes the client sees: the same for one contributor across this client's projects, different for every other client."
            : "Contributors are shown by a code that stays the same across your projects. Names and contact details are never shared."}
        </p>
      </div>
      <Card className="grid gap-5 p-5">
        <StatGroup>
          <Stat label="Contributors" value={n(w.contributors)} />
          <Stat label="Active in the last 7 days" value={n(w.active_last_7_days)} />
        </StatGroup>
        {[
          ["Languages", langs],
          ["Domains", domains],
        ].map(([title, list]) =>
          (list as typeof langs).length ? (
            <div key={title as string} className="grid gap-2 text-sm">
              <h3 className="text-fg-muted">{title as string}</h3>
              <ul className="flex flex-wrap gap-1.5">
                {(list as typeof langs).map((s) => (
                  <li key={s.value}>
                    <Tag className="gap-1.5">
                      {labelFor(s.value)}
                      <span className="font-mono text-fg-muted">{s.contributors}</span>
                      <span className="sr-only">
                        {s.contributors === 1 ? " contributor" : " contributors"}
                      </span>
                    </Tag>
                  </li>
                ))}
              </ul>
            </div>
          ) : null,
        )}
      </Card>
      {w.workers.length > 0 && (
        <Table
          caption="Contributors on this project"
          columns={workerColumns}
          rows={w.workers}
          rowKey={(x) => x.code}
          empty="No answers yet."
        />
      )}
    </section>
  );
}
