"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { LineChart } from "@/components/charts/line-chart";
import { ButtonLink } from "@/components/ui/button";
import { Alert, Card, EmptyState, Skeleton, Tag } from "@/components/ui/display";
import { TextLink } from "@/components/ui/text-link";
import { labelFor } from "@/content/profile";
import { api, type EarningsSummary, type MyQuality, type WorkProject } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import {
  formatDay,
  goldTrend,
  mainTotals,
  minutesText,
  projectsWithWork,
  payoutNote,
  percent,
  score,
  tasksLeftText,
  trustExplanation,
  trustInputs,
  trustState,
} from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import { formatKes, taskTypeLabel } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { useProfile } from "./portal";
import { StatusBadge } from "./status";

type Feedback = {
  id: string;
  body: string;
  project_name: string;
  automatic: boolean;
  read_at: string | null;
  created_at: string;
};

const skillText = (s: WorkProject["skills"][number]) =>
  s.kind === "language"
    ? labelFor(s.value)
    : s.kind === "task_type"
      ? (taskTypeLabel[s.value as keyof typeof taskTypeLabel] ?? s.value)
      : labelFor(s.value);

const sectionTitle = "font-display text-lg font-semibold";

/**
 * The dashboard once a contributor is active (DESIGN.md section 11): work on
 * offer on the left; trust score, earnings and feedback on the right; the
 * gold accuracy trend below. A paused contributor sees the same record
 * without work.
 */
export function WorkingDashboard({ paused }: { paused: boolean }) {
  const { profile } = useProfile();
  const projects = useLoad(() =>
    paused
      ? Promise.resolve([] as WorkProject[])
      : api<{ projects: WorkProject[] }>("GET", "/v1/me/projects").then((r) => r.projects),
  );
  const quality = useLoad(() => api<MyQuality>("GET", "/v1/me/quality"));
  const earnings = useLoad(() => api<EarningsSummary>("GET", "/v1/me/earnings"));
  const feedback = useLoad(() =>
    api<{ feedback: Feedback[] }>("GET", "/v1/me/feedback").then((r) => r.feedback),
  );
  const first = profile.display_name.split(/\s+/)[0];
  const count = projects.status === "ready" ? projectsWithWork(projects.data).open : null;

  return (
    <div className="grid gap-10">
      <header className="grid gap-3">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          Welcome, {first}
        </h1>
        <p className="flex flex-wrap items-center gap-x-3 gap-y-2 text-fg-muted">
          <StatusBadge status={profile.status} />
          <span className="font-mono text-sm">{profile.public_code}</span>
          {profile.languages.length > 0 && (
            <span>{profile.languages.map((l) => labelFor(l.language)).join(", ")}</span>
          )}
          {count != null && !paused && (
            <span>
              {count} {count === 1 ? "project" : "projects"} available
            </span>
          )}
        </p>
      </header>

      {paused && (
        <Alert tone="warning" title="Your account is paused">
          You cannot take tasks while paused. Your earnings and quality record stay here, and approved work is
          still paid. The MezAI team will contact you by email about next steps.
        </Alert>
      )}

      <div className="grid gap-10 lg:grid-cols-12">
        <section aria-labelledby="work-h" className="grid content-start gap-4 lg:col-span-7">
          <div className="flex items-baseline justify-between gap-4">
            <h2 id="work-h" className={sectionTitle}>
              Work available
            </h2>
            {!paused && <TextLink href="/c/work">All tasks</TextLink>}
          </div>
          <ProjectRows state={projects} paused={paused} />
        </section>

        <aside className="grid content-start gap-6 lg:col-span-5" aria-label="Your record">
          <TrustPanel state={quality} />
          <EarningsCard state={earnings} />
          <FeedbackList state={feedback} />
        </aside>
      </div>

      <TrendSection state={quality} />
    </div>
  );
}

type Loaded<T> = ReturnType<typeof useLoad<T>>;

function Failed({ error }: { error: Parameters<typeof errorMessage>[0] }) {
  return <Alert tone="danger" title={errorMessage(error)} />;
}

function ProjectRows({ state, paused }: { state: Loaded<WorkProject[]>; paused: boolean }) {
  if (state.status === "loading") return <Skeleton className="h-56 w-full" />;
  if (state.status === "error")
    return state.error.code === "not_eligible" ? <NoWork paused={paused} /> : <Failed error={state.error} />;
  const { shown, open } = projectsWithWork(state.data);
  if (open === 0) return <NoWork paused={paused} />;
  return (
    <>
      <ul
        className="divide-y divide-border rounded-card border border-border bg-surface"
        aria-label="Projects"
      >
        {shown.map((p) => {
          const facts = [minutesText(p.minutes_per_task), tasksLeftText(p.tasks_left)].filter(Boolean);
          return (
            <li key={p.id} className="grid gap-3 p-5 sm:grid-cols-[1fr_auto] sm:items-center">
              <div className="grid min-w-0 gap-1.5">
                <h3 className="font-medium">
                  <Link href={`/c/projects/${p.id}`} className="underline-offset-4 hover:underline">
                    {p.name}
                  </Link>
                </h3>
                <p className="flex flex-wrap items-baseline gap-x-3 gap-y-1 text-sm text-fg-muted">
                  <span className="font-mono font-medium text-fg">
                    {formatKes(p.pay_per_task_cents, p.currency)}
                  </span>
                  <span>a task</span>
                  <span aria-hidden="true">·</span>
                  <span>{taskTypeLabel[p.task_type]}</span>
                  {facts.map((f) => (
                    <span key={f} className="flex gap-3">
                      <span aria-hidden="true">·</span>
                      {f}
                    </span>
                  ))}
                </p>
                {p.skills.length > 0 && (
                  <ul className="flex flex-wrap gap-1.5" aria-label="Skills needed">
                    {p.skills.map((s) => (
                      <li key={`${s.kind}:${s.value}`}>
                        <Tag>{skillText(s)}</Tag>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              <ButtonLink href={`/c/work/${p.id}`} variant="secondary" size="sm" arrow>
                Start<span className="sr-only"> {p.name}</span>
              </ButtonLink>
            </li>
          );
        })}
      </ul>
      {open > shown.length && (
        <p className="text-sm text-fg-muted">
          Showing {shown.length} of {open} projects with work.{" "}
          <TextLink href="/c/work">See all tasks</TextLink>
        </p>
      )}
    </>
  );
}

function NoWork({ paused }: { paused: boolean }) {
  return (
    <EmptyState
      title={paused ? "Tasks are closed while you are paused" : "No open project needs your skills today"}
      action={
        paused ? undefined : (
          <ButtonLink href="/c/qualify" variant="secondary">
            Qualification tests
          </ButtonLink>
        )
      }
    >
      {paused
        ? "Your record and earnings stay visible below."
        : "New projects appear here when they open. Another qualification test widens what you can work on."}
    </EmptyState>
  );
}

/** The trust score on a charcoal panel: the one place lime marks "on track". */
function TrustPanel({ state }: { state: Loaded<MyQuality> }) {
  if (state.status === "loading") return <Skeleton className="h-72 w-full" />;
  if (state.status === "error") return <Failed error={state.error} />;
  const q = state.data;
  const st = trustState(q);
  return (
    <section
      data-theme="dark"
      aria-labelledby="trust-h"
      className="grid gap-5 rounded-card bg-bg p-6 text-fg"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="grid gap-1">
          <h2 id="trust-h" className={sectionTitle}>
            Trust score
          </h2>
          <p className="text-xs text-fg-muted">
            As of {new Date(q.as_of).toLocaleDateString("en-GB", { day: "numeric", month: "short" })}, start
            of day · last {q.window_days} days
          </p>
        </div>
        {st === "building" && <Tag>Building</Tag>}
      </div>
      <p className="flex items-center gap-3">
        <span className="font-mono text-6xl font-medium tracking-tight tabular">{score(q.trust?.score)}</span>
        {st === "good" && (
          <span className="size-3 rounded-full bg-lime" aria-hidden="true" title="Above the floor" />
        )}
        <span className="sr-only">
          {st === "good" ? "Above the floor" : st === "low" ? "Below the floor" : ""}
        </span>
      </p>
      <p className="text-sm text-fg-muted">{trustExplanation(q)}</p>
      <dl className="grid gap-3 border-t border-border pt-4">
        {trustInputs(q).map((i) => (
          <div key={i.key} className="grid grid-cols-[1fr_auto] items-baseline gap-x-4">
            <dt className="text-sm">
              {i.label}
              <span className="block text-xs text-fg-muted">
                {i.hint} · weight {Math.round(i.weight * 100)}%
              </span>
            </dt>
            <dd className="font-mono text-lg tabular">{percent(i.value)}</dd>
          </div>
        ))}
      </dl>
      <p className="text-xs text-fg-muted">
        An input with no data yet is left out and the others carry its weight. Today&apos;s work shows
        tomorrow.{" "}
        <TextLink href="/c/training#quality" className="text-fg">
          How the score works
        </TextLink>
      </p>
    </section>
  );
}

function Money({
  label,
  cents,
  note,
  strong,
}: {
  label: string;
  cents: number;
  note: ReactNode;
  strong?: boolean;
}) {
  return (
    <div className="grid grid-cols-[1fr_auto] items-baseline gap-x-4 py-3 first:pt-0 last:pb-0">
      <dt className="text-sm">
        {label}
        <span className="block text-xs text-fg-muted">{note}</span>
      </dt>
      <dd className={cn("font-mono whitespace-nowrap tabular", strong ? "text-2xl font-medium" : "text-lg")}>
        {formatKes(cents)}
      </dd>
    </div>
  );
}

function EarningsCard({ state }: { state: Loaded<EarningsSummary> }) {
  if (state.status === "loading") return <Skeleton className="h-48 w-full" />;
  if (state.status === "error") return <Failed error={state.error} />;
  const s = state.data;
  const t = mainTotals(s);
  return (
    <section aria-labelledby="earnings-h">
      <Card className="grid gap-5 p-6">
        <div className="flex items-baseline justify-between gap-4">
          <h2 id="earnings-h" className={sectionTitle}>
            Earnings
          </h2>
          <TextLink href="/c/earnings">Statement</TextLink>
        </div>
        <dl className="grid divide-y divide-border">
          <Money label="Approved" cents={t.approved_cents} note="Accepted, not yet paid" strong />
          <Money
            label="Pending"
            cents={t.pending_cents}
            note={`Estimate · ${t.pending_tasks} ${t.pending_tasks === 1 ? "task" : "tasks"} in review`}
          />
          <Money label="Paid" cents={t.paid_cents} note="To your M-Pesa" />
        </dl>
        <p className="border-t border-border pt-4 text-sm text-fg-muted">{payoutNote(s)}</p>
      </Card>
    </section>
  );
}

function FeedbackList({ state }: { state: Loaded<Feedback[]> }) {
  if (state.status === "loading") return <Skeleton className="h-32 w-full" />;
  if (state.status === "error") return <Failed error={state.error} />;
  const latest = state.data.slice(0, 3);
  return (
    <section aria-labelledby="feedback-h" className="grid gap-3">
      <h2 id="feedback-h" className={sectionTitle}>
        Latest feedback
      </h2>
      {latest.length === 0 ? (
        <p className="text-sm text-fg-muted">
          No feedback yet. Reviewers&apos; notes and the daily note on missed checks appear here.
        </p>
      ) : (
        <ul className="grid gap-3">
          {latest.map((f) => (
            <li key={f.id} className="grid gap-1 border-l-2 border-border pl-4">
              <p className="text-xs text-fg-muted">
                {f.project_name} · {f.automatic ? "Daily check note" : "Reviewer"} ·{" "}
                {formatDay(f.created_at.slice(0, 10))}
                {!f.read_at && <span className="ml-2 font-semibold text-fg">New</span>}
              </p>
              <p className="text-sm">{f.body}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function TrendSection({ state }: { state: Loaded<MyQuality> }) {
  if (state.status !== "ready") return null;
  const pts = goldTrend(state.data);
  const startDay = pts[0]?.day;
  const label = (x: number) => {
    if (!startDay) return String(x);
    const d = new Date(Date.parse(`${startDay}T00:00:00Z`) + x * 86_400_000);
    return d.toLocaleDateString("en-GB", { day: "numeric", month: "short", timeZone: "UTC" });
  };
  return (
    <section aria-labelledby="trend-h" className="grid gap-4 border-t border-border pt-8">
      <h2 id="trend-h" className="sr-only">
        Gold accuracy trend
      </h2>
      {pts.length < 2 ? (
        <p className="text-sm text-fg-muted">
          <span className="font-display text-base font-semibold text-fg">Gold accuracy, last 30 days. </span>
          The trend appears after two days with scored checks.
        </p>
      ) : (
        <>
          <LineChart
            title="Gold accuracy, last 30 days"
            note="Share of known-answer checks you matched, end of each day"
            lines={[{ name: "Gold accuracy", points: pts, series: 1 }]}
            yDomain={[0, 1]}
            formatY={(y) => `${Math.round(y * 100)}%`}
            formatX={label}
            height={180}
          />
          <p className="sr-only">
            From {percent(pts[0].y)} on {formatDay(pts[0].day)} to {percent(pts[pts.length - 1].y)} on{" "}
            {formatDay(pts[pts.length - 1].day)}.
          </p>
        </>
      )}
    </section>
  );
}
