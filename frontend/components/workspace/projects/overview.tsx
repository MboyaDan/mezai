"use client";

import Link from "next/link";
import { Check, Circle } from "lucide-react";
import { Alert, Card, Skeleton, Tag } from "@/components/ui/display";
import { labelFor } from "@/content/profile";
import { api, type ImportJob, type ProjectSkill } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { formatKes, rateToPercent, readiness, taskTypeLabel } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "../contributors";
import { useCan } from "../workspace";
import { useProject } from "./frame";
import { ImportHistory } from "./imports";
import { Reviewers } from "./reviewers";

export function skillText(s: ProjectSkill): string {
  if (s.kind === "language")
    return `${labelFor(s.value)}${s.min_proficiency ? ` (${s.min_proficiency} or better)` : ""}`;
  if (s.kind === "task_type") return taskTypeLabel[s.value as keyof typeof taskTypeLabel] ?? s.value;
  return labelFor(s.value);
}

function Facts() {
  const { project: p } = useProject();
  const rows: Array<[string, React.ReactNode]> = [
    // For client users the API's count includes gold check items (A-033).
    [p.staff ? "Items" : "Items, with gold checks", p.item_count.toLocaleString("en-KE")],
    ["Target", p.target_count?.toLocaleString("en-KE") ?? "Not set"],
    ["Contributors per item", p.overlap_k],
    ["Gold mixed in", `${rateToPercent(p.gold_rate)}%`],
    ["Deadline", p.deadline ? shortDate(p.deadline) : "Not set"],
  ];
  if (p.staff) {
    rows.push(
      ["Active gold items", p.staff.gold_count],
      [
        "Pay per approved task",
        <span key="pay" className="font-mono tabular">
          {formatKes(p.staff.pay_per_task_cents, p.staff.currency)}
        </span>,
      ],
    );
  }
  if (p.published_at) rows.push(["Published", shortDate(p.published_at)]);
  if (p.closed_at) rows.push(["Closed", shortDate(p.closed_at)]);
  return (
    <Card className="p-5">
      <h2 className="font-display font-semibold">Set-up</h2>
      <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-sm">
        {rows.map(([k, v]) => (
          <div key={k} className="contents">
            <dt className="text-fg-muted">{k}</dt>
            <dd className="text-right font-mono tabular">{v}</dd>
          </div>
        ))}
      </dl>
      {p.skills.length > 0 && (
        <div className="mt-4 grid gap-2 text-sm">
          <p className="text-fg-muted">Required skills</p>
          <ul className="flex flex-wrap gap-1.5">
            {p.skills.map((s) => (
              <li key={`${s.kind}/${s.value}`}>
                <Tag>{skillText(s)}</Tag>
              </li>
            ))}
          </ul>
        </div>
      )}
    </Card>
  );
}

function Readiness() {
  const { project: p } = useProject();
  const can = useCan();
  if (!p.staff || p.status === "live" || p.status === "closed") return null;
  const steps = readiness(p);
  const next = p.status === "draft" ? "mark it ready" : "publish it";
  const links: Record<string, string> = {
    instructions: `/w/projects/${p.id}/settings`,
    items: `/w/projects/${p.id}/items`,
    gold: `/w/projects/${p.id}/gold`,
  };
  return (
    <Card className="p-5">
      <h2 className="font-display font-semibold">Before you {next}</h2>
      <ul className="mt-3 grid gap-2 text-sm">
        {steps.map((s) => (
          <li key={s.key} className="flex items-center gap-2">
            {s.done ? (
              <Check aria-hidden="true" className="size-4 text-success" />
            ) : (
              <Circle aria-hidden="true" className="size-4 text-fg-muted" />
            )}
            <span className="sr-only">{s.done ? "Done:" : "To do:"}</span>
            {s.done || !can("project.manage", "item.import", "gold.manage") ? (
              <span className={s.done ? "text-fg" : "text-fg-muted"}>{s.label}</span>
            ) : (
              <Link href={links[s.key]} className="underline underline-offset-4">
                {s.label}
              </Link>
            )}
          </li>
        ))}
      </ul>
      {p.status === "draft" && (
        <p className="mt-3 text-sm text-fg-muted">
          Marking ready needs instructions; publishing needs all of the above.
        </p>
      )}
    </Card>
  );
}

function Files() {
  const { project: p } = useProject();
  const list = useLoad(
    () =>
      api<{ imports: ImportJob[] }>("GET", `/v1/projects/${p.id}/imports`).then((r) =>
        r.imports.filter((j) => j.kind === "items"),
      ),
    `${p.id}/${p.item_count}`,
  );
  return (
    <section aria-labelledby="files-h" className="grid gap-3">
      <h2 id="files-h" className="font-display font-semibold">
        Item files
      </h2>
      {list.status === "loading" && <Skeleton className="h-24 w-full" />}
      {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
      {list.status === "ready" && <ImportHistory jobs={list.data} />}
    </section>
  );
}

export function ProjectOverview() {
  const { project: p } = useProject();
  const can = useCan();
  return (
    <div className="grid gap-6 lg:grid-cols-3">
      <div className="grid content-start gap-6 lg:col-span-2">
        <Alert tone="info" title="Progress and quality figures start when contributors begin work">
          Completed items, throughput, gold accuracy and agreement appear here once the project is live and
          submissions arrive.
        </Alert>
        <section aria-labelledby="instr-h" className="grid gap-3">
          <h2 id="instr-h" className="font-display font-semibold">
            Contributor instructions
          </h2>
          {p.instructions_md.trim() ? (
            <Card className="max-h-96 overflow-y-auto p-5 text-sm leading-relaxed whitespace-pre-wrap">
              {p.instructions_md}
            </Card>
          ) : (
            <p className="text-sm text-fg-muted">Not written yet.</p>
          )}
        </section>
        <Files />
      </div>
      <div className="grid content-start gap-6">
        <Readiness />
        <Facts />
        {can("project.manage") && <Reviewers />}
      </div>
    </div>
  );
}
