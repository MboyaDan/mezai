"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ButtonLink } from "@/components/ui/button";
import { Alert, Breadcrumb, Card, EmptyState, Skeleton, Tag } from "@/components/ui/display";
import { labelFor } from "@/content/profile";
import { api, type ApiError, type WorkProject } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { minutesText, tasksLeftText } from "@/lib/earnings";
import { formatKes, taskTypeLabel } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { useProfile } from "./portal";

const skillText = (s: WorkProject["skills"][number]) =>
  s.kind === "language"
    ? `${labelFor(s.value)}${s.min_proficiency ? ` (${s.min_proficiency} or better)` : ""}`
    : s.kind === "task_type"
      ? (taskTypeLabel[s.value as keyof typeof taskTypeLabel] ?? s.value)
      : labelFor(s.value);

/** Why there is no work yet, in the contributor's words. */
function NotYet({ error }: { error?: ApiError }) {
  const { profile } = useProfile();
  const why =
    profile.status !== "active"
      ? "Tasks open once you pass a qualification test and your details are checked."
      : profile.identity_status !== "verified"
        ? "Tasks open once the MezAI team has checked your details."
        : error
          ? errorMessage(error)
          : "No open project needs your qualified skills at the moment. Take another qualification test to widen what you can work on.";
  return (
    <EmptyState
      title="No tasks for you yet"
      action={
        <ButtonLink href={profile.status === "active" ? "/c/qualify" : "/c"} variant="secondary">
          {profile.status === "active" ? "Qualification tests" : "Back to dashboard"}
        </ButtonLink>
      }
    >
      {why}
    </EmptyState>
  );
}

/** /c/work: the live projects this contributor may work on. */
export function WorkList() {
  const list = useLoad(() =>
    api<{ projects: WorkProject[] }>("GET", "/v1/me/projects").then((r) => r.projects),
  );
  return (
    <div className="grid gap-8">
      <header className="grid gap-2">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          Tasks
        </h1>
        <p className="max-w-2xl text-fg-muted">
          Projects that match your qualified skills. You work on one task at a time, and every approved task
          is paid at the rate shown.
        </p>
      </header>
      {list.status === "loading" && <Skeleton className="h-48 w-full" />}
      {list.status === "error" &&
        (list.error.code === "not_eligible" ? (
          <NotYet />
        ) : (
          <Alert tone="danger" title={errorMessage(list.error)} />
        ))}
      {list.status === "ready" && list.data.length === 0 && <NotYet />}
      {list.status === "ready" && list.data.length > 0 && (
        <ul className="grid gap-3" aria-label="Projects">
          {list.data.map((p) => (
            <li key={p.id}>
              <Card className="grid gap-4 p-5 md:grid-cols-[1fr_auto] md:items-center">
                <div className="grid min-w-0 gap-2">
                  <h2 className="font-display text-lg font-semibold">
                    <Link href={`/c/projects/${p.id}`} className="underline-offset-4 hover:underline">
                      {p.name}
                    </Link>
                  </h2>
                  <p className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-fg-muted">
                    <span>{taskTypeLabel[p.task_type]}</span>
                    <span className="font-mono text-fg">
                      {formatKes(p.pay_per_task_cents, p.currency)} a task
                    </span>
                    <span>{p.my_submitted} submitted by you</span>
                    {minutesText(p.minutes_per_task) && <span>{minutesText(p.minutes_per_task)}</span>}
                    {tasksLeftText(p.tasks_left) && <span>{tasksLeftText(p.tasks_left)}</span>}
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
                <div className="flex flex-wrap gap-2">
                  <ButtonLink href={`/c/projects/${p.id}`} variant="ghost" size="sm">
                    Brief<span className="sr-only"> for {p.name}</span>
                  </ButtonLink>
                  <ButtonLink href={`/c/work/${p.id}`} variant="secondary" size="sm" arrow>
                    Start<span className="sr-only"> {p.name}</span>
                  </ButtonLink>
                </div>
              </Card>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** What the automatic checks look for, in plain words. */
export function checkList(p: WorkProject): string[] {
  const out: string[] = [];
  if (p.task_type === "pairwise" && p.min_rationale_chars > 0)
    out.push(`Your reason for a choice must be at least ${p.min_rationale_chars} characters.`);
  if (p.qa_checks.block_unchanged_translation)
    out.push("If you score a translation below 5, correct it before submitting.");
  if (p.qa_checks.warn_fast_seconds > 0)
    out.push(`Tasks finished in under ${p.qa_checks.warn_fast_seconds} seconds are flagged for a reviewer.`);
  if (p.qa_checks.warn_rationale_mismatch)
    out.push("A reason that praises the response you did not choose is flagged for a reviewer.");
  if (p.qa_checks.warn_pasted) out.push("Written answers that are mostly pasted are flagged for a reviewer.");
  return out;
}

/** /c/projects/[id]: the project brief before starting. */
export function ProjectBrief() {
  const { id } = useParams<{ id: string }>();
  const p = useLoad(() => api<WorkProject>("GET", `/v1/me/projects/${id}`), id);
  if (p.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (p.status === "error")
    return p.error.code === "not_eligible" || p.error.code === "not_found" ? (
      <NotYet error={p.error} />
    ) : (
      <Alert tone="danger" title={errorMessage(p.error)} />
    );
  const pr = p.data;
  const checks = checkList(pr);
  return (
    <div className="grid max-w-3xl gap-8">
      <Breadcrumb items={[{ label: "Tasks", href: "/c/work" }, { label: pr.name }]} />
      <header className="grid gap-3">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          {pr.name}
        </h1>
        <dl className="flex flex-wrap gap-x-6 gap-y-2 text-sm">
          <div className="flex gap-2">
            <dt className="text-fg-muted">Task type</dt>
            <dd>{taskTypeLabel[pr.task_type]}</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-fg-muted">Pay</dt>
            <dd className="font-mono">{formatKes(pr.pay_per_task_cents, pr.currency)} a task</dd>
          </div>
          <div className="flex gap-2">
            <dt className="text-fg-muted">Time per task</dt>
            <dd>{pr.reservation_minutes} minutes before it returns to the queue</dd>
          </div>
        </dl>
      </header>
      <Card className="p-5">
        <h2 className="font-display text-lg font-semibold">Instructions</h2>
        <p className="mt-2 whitespace-pre-wrap">{pr.instructions_md || "No instructions yet."}</p>
      </Card>
      {checks.length > 0 && (
        <section aria-labelledby="checks-h" className="grid gap-2">
          <h2 id="checks-h" className="font-display text-lg font-semibold">
            Checked before you submit
          </h2>
          <ul className="grid list-disc gap-1.5 pl-5 text-sm text-fg-muted">
            {checks.map((c) => (
              <li key={c}>{c}</li>
            ))}
          </ul>
        </section>
      )}
      <ul className="grid gap-1.5 text-sm text-fg-muted">
        <li>
          Some tasks are checks with known answers. You will not know which, so treat every task the same.
        </li>
        <li>
          Skip any task you cannot do well, and report anything harmful. You never have to finish a task that
          distresses you.
        </li>
        <li>Task content is confidential: do not copy it or paste it anywhere else.</li>
      </ul>
      <div>
        <ButtonLink href={`/c/work/${pr.id}`} arrow>
          Start working
        </ButtonLink>
      </div>
    </div>
  );
}
