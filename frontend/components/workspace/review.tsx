"use client";

import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useId, useRef, useState } from "react";
import { AnswerInput } from "@/components/qualification/answer-input";
import { AnswerSummary, QuestionContent } from "@/components/qualification/content";
import { Button, ButtonLink } from "@/components/ui/button";
import { Checkbox, ChoiceGroup, Radio } from "@/components/ui/choice";
import { Alert, Breadcrumb, Card, Skeleton, StatusPill, Tag } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Select, Textarea } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import {
  ApiError,
  api,
  type DecisionResult,
  type ReviewItem,
  type ReviewQueue as Queue,
  type ReviewSubmission,
  type RouteReason,
  type TaskConfig,
} from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { errorMessage, type Answer, type QuestionType } from "@/lib/onboarding";
import { taskTypeLabel } from "@/lib/projects";
import {
  buildDecision,
  decisionKey,
  decisionLabel,
  duration,
  emptyDraft,
  reasonHelp,
  reasonLabel,
  type Draft,
  type DraftErrors,
} from "@/lib/review";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { PageTitle, useCan } from "./workspace";

const reasons = Object.keys(reasonLabel) as RouteReason[];
const pct = (v: number | null | undefined) => (v == null ? "–" : `${Math.round(v * 100)}%`);

function queuePath(project: string, reason: string) {
  const q = new URLSearchParams();
  if (project) q.set("project_id", project);
  if (reason) q.set("reason", reason);
  const s = q.toString();
  return `/v1/review/items${s ? `?${s}` : ""}` as const;
}

/** /w/review: items the quality engine sent to a person (brief 8.3), oldest first. */
export function ReviewQueue() {
  const params = useSearchParams();
  const router = useRouter();
  const project = params.get("project") ?? "";
  const reason = params.get("reason") ?? "";
  const list = useLoad(() => api<Queue>("GET", queuePath(project, reason)), `${project}|${reason}`);
  const can = useCan();

  function setFilter(key: "project" | "reason", value: string) {
    const q = new URLSearchParams(params.toString());
    if (value) q.set(key, value);
    else q.delete(key);
    router.replace(`/w/review${q.size ? `?${q}` : ""}`);
  }

  return (
    <>
      <PageTitle title="Review">
        Items where contributors disagreed, an answer carries a QA warning, or the audit sample picked them.
        Compare the answers side by side and decide the final one.{" "}
        {can("review.adjudicate")
          ? "You see every project."
          : "You see the projects you are assigned to review."}
      </PageTitle>
      <div className="grid gap-4">
        <div className="flex flex-wrap gap-4">
          <Field label="Project" className="w-full max-w-72">
            <Select value={project} onChange={(e) => setFilter("project", e.target.value)}>
              <option value="">All projects</option>
              {list.status === "ready" &&
                list.data.projects.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name} ({p.waiting})
                  </option>
                ))}
            </Select>
          </Field>
          <Field label="Why it is here" className="w-full max-w-56">
            <Select value={reason} onChange={(e) => setFilter("reason", e.target.value)}>
              <option value="">Any reason</option>
              {reasons.map((r) => (
                <option key={r} value={r}>
                  {reasonLabel[r]}
                </option>
              ))}
            </Select>
          </Field>
        </div>
        {list.status === "loading" && <Skeleton className="h-40 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" && (
          <Table
            caption="Items waiting for review, oldest first"
            hideCaption
            rows={list.data.items}
            rowKey={(r) => r.item_id}
            empty={
              project || reason
                ? "Nothing waiting with these filters."
                : "Nothing to review. Items appear here when the quality engine routes them."
            }
            columns={[
              {
                key: "since",
                header: "Waiting since",
                cell: (r) => shortDate(r.waiting_since),
                sortValue: (r) => r.waiting_since,
              },
              {
                key: "project",
                header: "Project",
                cell: (r) => (
                  <span className="grid gap-0.5">
                    <span>{r.project_name}</span>
                    <span className="text-sm text-fg-muted">{r.org_name}</span>
                  </span>
                ),
                sortValue: (r) => r.project_name,
              },
              {
                key: "item",
                header: "Item",
                cell: (r) => <span className="font-mono text-xs">{r.external_id ?? "–"}</span>,
              },
              {
                key: "reason",
                header: "Reason",
                cell: (r) => <Tag>{reasonLabel[r.reason]}</Tag>,
                sortValue: (r) => r.reason,
              },
              {
                key: "agree",
                header: "Agreement",
                cell: (r) => <span className="font-mono text-sm">{pct(r.agreement)}</span>,
              },
              {
                key: "go",
                header: <span className="sr-only">Open</span>,
                cell: (r) => (
                  <ButtonLink size="sm" variant="secondary" href={`/w/review/${r.item_id}`}>
                    Review<span className="sr-only"> item {r.external_id ?? ""}</span>
                  </ButtonLink>
                ),
              },
            ]}
          />
        )}
      </div>
    </>
  );
}

/** /w/review/[id]: one item, every answer side by side, and the decision. */
export function ReviewItemView() {
  const { id } = useParams<{ id: string }>();
  const item = useLoad(() => api<ReviewItem>("GET", `/v1/review/items/${id}`), id);
  if (item.status === "loading") return <Skeleton className="h-96 w-full" />;
  if (item.status === "error") return <Alert tone="danger" title={errorMessage(item.error)} />;
  const it = item.data;
  const type = it.task_type as QuestionType;
  const config = it.config as TaskConfig;
  const reason = it.consensus?.reason ?? null;

  return (
    <div className="grid gap-6">
      <Breadcrumb
        items={[
          { label: "Review", href: "/w/review" },
          { label: it.project_name, href: `/w/review?project=${it.project_id}` },
          { label: it.external_id ?? "Item" },
        ]}
      />
      <header className="grid gap-2">
        <h1 className="font-display text-2xl font-semibold tracking-[-0.01em] break-words">
          Item <span className="font-mono text-xl">{it.external_id ?? it.id.slice(0, 8)}</span>
        </h1>
        <div className="flex flex-wrap items-center gap-2 text-sm text-fg-muted">
          <Tag>{taskTypeLabel[it.task_type as keyof typeof taskTypeLabel] ?? it.task_type}</Tag>
          {reason && <Tag>{reasonLabel[reason]}</Tag>}
          <span>{it.project_name}</span>
          {it.consensus?.agreement != null && <span>· Agreement {pct(it.consensus.agreement)}</span>}
          {it.consensus?.spread != null && <span>· Spread {it.consensus.spread} points</span>}
        </div>
        {reason && <p className="max-w-prose text-sm text-fg-muted">{reasonHelp[reason]}</p>}
      </header>

      <section aria-labelledby="content-h" className="grid gap-3">
        <h2 id="content-h" className="font-display font-semibold">
          The item
        </h2>
        <QuestionContent type={type} content={it.content} />
        {it.instructions_md.trim() && (
          <details className="rounded-card border border-border p-4 text-sm">
            <summary className="cursor-pointer font-medium">Project instructions</summary>
            <div className="mt-3 max-h-80 overflow-y-auto leading-relaxed whitespace-pre-wrap">
              {it.instructions_md}
            </div>
          </details>
        )}
      </section>

      {it.decision ? (
        <Decided item={it} type={type} />
      ) : it.status === "in_review" ? (
        <DecisionForm item={it} type={type} config={config} onDone={item.reload} />
      ) : (
        <Alert tone="info" title="This item is not waiting for review." />
      )}
    </div>
  );
}

function SubmissionHead({ s, index }: { s: ReviewSubmission; index: number }) {
  return (
    <div className="flex flex-wrap items-baseline justify-between gap-2">
      <p className="font-medium">
        Answer {index + 1} <span className="font-mono text-xs text-fg-muted">{s.contributor}</span>
      </p>
      <span className="font-mono text-xs text-fg-muted">{duration(s.time_on_task_ms)} on task</span>
    </div>
  );
}

function Warnings({ s }: { s: ReviewSubmission }) {
  if (!s.warnings?.length) return null;
  return (
    <ul className="grid gap-1 text-sm text-warning">
      {s.warnings.map((w, i) => (
        <li key={i}>QA warning: {w.message}</li>
      ))}
    </ul>
  );
}

function DecisionForm({
  item,
  type,
  config,
  onDone,
}: {
  item: ReviewItem;
  type: QuestionType;
  config: TaskConfig;
  onDone: () => void;
}) {
  const can = useCan();
  const toast = useToast();
  const router = useRouter();
  const [d, setD] = useState<Draft>(emptyDraft);
  const [errors, setErrors] = useState<DraftErrors>({});
  const [apiError, setApiError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  // One key per decision: a retry after a network failure replays, never decides twice.
  const key = useRef(decisionKey(item.id));
  // Radio groups need names unique on the page: Next keeps the previous
  // item mounted (hidden) after moving on, and same-named radios would share a group.
  const group = useId();
  const adjudicate = item.can_adjudicate;
  const consensusOk = !!item.consensus?.complete;
  const update = (patch: Partial<Draft>) => setD((cur) => ({ ...cur, ...patch }));

  function toggleReject(id: string, on: boolean) {
    const next = { ...d.rejections };
    if (on) next[id] = "";
    else delete next[id];
    update({ rejections: next });
  }

  async function submit() {
    const { body, errors: errs } = buildDecision(d);
    setErrors(errs);
    setApiError(null);
    if (!body) {
      document.querySelector<HTMLElement>("[data-review-error]")?.scrollIntoView({ block: "center" });
      return;
    }
    setBusy(true);
    try {
      const r = await api<DecisionResult>("POST", `/v1/review/items/${item.id}/decision`, body, {
        "Idempotency-Key": key.current,
      });
      toast({
        tone: "success",
        title: r.item_status === "final" ? "Final answer recorded" : "Item rejected",
      });
      // Straight on to the next item on this project, if there is one.
      const q = await api<Queue>("GET", queuePath(item.project_id, "")).catch(() => null);
      const next = q?.items.find((x) => x.item_id !== item.id);
      if (next) router.push(`/w/review/${next.item_id}`);
      else onDone();
    } catch (e) {
      const err = e instanceof ApiError ? e : new ApiError(0, "network");
      if (err.code === "already_decided") onDone();
      setApiError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <section aria-labelledby="answers-h" className="grid gap-3">
        <h2 id="answers-h" className="font-display font-semibold">
          Answers ({item.submissions.length})
        </h2>
        <p className="max-w-prose text-sm text-fg-muted">
          Every answer is accepted and paid unless you mark it as not accepted. Do that only for careless work
          or ignored instructions, not for honest disagreement. The contributor sees your reason.
        </p>
        <ol className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {item.submissions.map((s, i) => {
            const rejected = s.id in d.rejections;
            const chosen = d.decision === "choose_submission" && d.chosen === s.id;
            return (
              <li key={s.id}>
                <Card className={cn("grid gap-3 p-4", chosen && "border-fg", rejected && "border-danger")}>
                  <SubmissionHead s={s} index={i} />
                  <div className="text-[0.9375rem]">
                    <AnswerSummary type={type} answer={s.answer as Answer} />
                  </div>
                  <Warnings s={s} />
                  <Radio
                    name={`chosen-${group}`}
                    checked={chosen}
                    onChange={() => update({ decision: "choose_submission", chosen: s.id })}
                    label="Use as the final answer"
                  />
                  <Checkbox
                    checked={rejected}
                    onChange={(e) => toggleReject(s.id, e.target.checked)}
                    label="Do not accept this answer"
                    hint="It will not earn."
                  />
                  {rejected && (
                    <Field label="Reason for the contributor" error={errors[`reject-${s.id}`]} required>
                      <Textarea
                        data-review-error={errors[`reject-${s.id}`] ? true : undefined}
                        rows={2}
                        maxLength={1000}
                        value={d.rejections[s.id]}
                        onChange={(e) => update({ rejections: { ...d.rejections, [s.id]: e.target.value } })}
                      />
                    </Field>
                  )}
                  {can("feedback.write") && (
                    <Field label="Feedback" hint="Sent to the contributor.">
                      <Textarea
                        rows={2}
                        maxLength={1000}
                        value={d.feedback[s.id] ?? ""}
                        onChange={(e) => update({ feedback: { ...d.feedback, [s.id]: e.target.value } })}
                      />
                    </Field>
                  )}
                </Card>
              </li>
            );
          })}
        </ol>
      </section>

      <section aria-labelledby="decide-h" className="grid gap-4">
        <h2 id="decide-h" className="font-display font-semibold">
          Decision
        </h2>
        {item.consensus && (
          <Card className="grid gap-1 p-4 text-sm">
            <p className="font-medium">Consensus</p>
            {consensusOk ? (
              <AnswerSummary type={type} answer={item.consensus.result as Answer} />
            ) : (
              <p className="text-fg-muted">No consensus answer. Choose an answer or write the final one.</p>
            )}
          </Card>
        )}
        <div data-review-error={errors.decision ? true : undefined}>
          <ChoiceGroup legend="Final answer" error={errors.decision}>
            <Radio
              name={`decision-${group}`}
              checked={d.decision === "accept_consensus"}
              disabled={!consensusOk}
              onChange={() => update({ decision: "accept_consensus" })}
              label="Accept the consensus"
              hint={consensusOk ? "The answer above becomes final." : "Not available: there is no consensus."}
            />
            <Radio
              name={`decision-${group}`}
              checked={d.decision === "choose_submission"}
              onChange={() => update({ decision: "choose_submission" })}
              label="Use one contributor's answer"
              hint="Pick it with “Use as the final answer” on its card."
            />
            {adjudicate && (
              <Radio
                name={`decision-${group}`}
                checked={d.decision === "write_final"}
                onChange={() => update({ decision: "write_final" })}
                label="Write the final answer"
              />
            )}
            {adjudicate && (
              <Radio
                name={`decision-${group}`}
                checked={d.decision === "reject_item"}
                onChange={() => update({ decision: "reject_item" })}
                label="Reject the item"
                hint="For broken or unsuitable source data. It gets no final answer and is left out of the delivery. Honest answers on it are still paid."
              />
            )}
          </ChoiceGroup>
        </div>
        {d.decision === "write_final" && (
          <Card className="grid gap-3 p-4" data-review-error={errors.final ? true : undefined}>
            <p className="font-medium">Final answer</p>
            {errors.final && <p className="text-sm text-danger">{errors.final}</p>}
            <AnswerInput
              type={type}
              config={config}
              content={item.content}
              value={d.final}
              onChange={(a) => update({ final: a })}
            />
          </Card>
        )}
        <Field
          label={d.decision === "reject_item" ? "Reason" : "Note"}
          hint={
            d.decision === "reject_item"
              ? "Kept in the audit log. At least 10 characters."
              : "Kept in the audit log, not shown to contributors."
          }
          error={errors.reason}
          required={d.decision === "reject_item"}
        >
          <Textarea
            data-review-error={errors.reason ? true : undefined}
            rows={2}
            maxLength={1000}
            value={d.reason}
            onChange={(e) => update({ reason: e.target.value })}
          />
        </Field>
        {apiError && <Alert tone="danger" title={errorMessage(apiError)} />}
        <div className="flex flex-wrap items-center gap-3">
          <Button
            onClick={submit}
            loading={busy}
            variant={d.decision === "reject_item" ? "danger" : "primary"}
          >
            {d.decision === "reject_item" ? "Reject item" : "Record decision"}
          </Button>
          <Link
            href={`/w/review?project=${item.project_id}`}
            className="text-sm underline underline-offset-4"
          >
            Back to the queue
          </Link>
        </div>
      </section>
    </>
  );
}

function Decided({ item, type }: { item: ReviewItem; type: QuestionType }) {
  const d = item.decision!;
  return (
    <>
      <Alert tone="success" title={decisionLabel[d.decision]}>
        {d.reviewer} on {shortDate(d.created_at)}
        {d.reason ? `. Note: ${d.reason}` : "."}
      </Alert>
      {d.final_answer && (
        <Card className="grid gap-1 p-4 text-sm">
          <p className="font-medium">Final answer</p>
          <AnswerSummary type={type} answer={d.final_answer as Answer} />
        </Card>
      )}
      <section aria-labelledby="answers-h" className="grid gap-3">
        <h2 id="answers-h" className="font-display font-semibold">
          Answers ({item.submissions.length})
        </h2>
        <ol className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {item.submissions.map((s, i) => (
            <li key={s.id}>
              <Card className="grid gap-3 p-4">
                <SubmissionHead s={s} index={i} />
                <AnswerSummary type={type} answer={s.answer as Answer} />
                <div className="flex flex-wrap gap-2">
                  <StatusPill
                    status={
                      s.status === "accepted" ? "success" : s.status === "rejected" ? "danger" : "neutral"
                    }
                  >
                    {s.status === "accepted" ? "Accepted" : s.status === "rejected" ? "Not accepted" : "Open"}
                  </StatusPill>
                  {s.agrees_with_final != null && (
                    <StatusPill status="neutral">
                      {s.agrees_with_final ? "Agrees with final" : "Differs from final"}
                    </StatusPill>
                  )}
                </div>
                {s.rejection_reason && <p className="text-sm text-fg-muted">Reason: {s.rejection_reason}</p>}
              </Card>
            </li>
          ))}
        </ol>
      </section>
      <div>
        <ButtonLink variant="secondary" href={`/w/review?project=${item.project_id}`}>
          Back to the queue
        </ButtonLink>
      </div>
    </>
  );
}
