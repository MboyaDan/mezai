"use client";

import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { BookOpen, Flag, Keyboard, SkipForward } from "lucide-react";
import { AnswerInput } from "@/components/qualification/answer-input";
import { QuestionContent } from "@/components/qualification/content";
import { Button, ButtonLink } from "@/components/ui/button";
import { ChoiceGroup, Radio } from "@/components/ui/choice";
import { Dialog, Drawer } from "@/components/ui/dialog";
import { Alert, Breadcrumb, EmptyState, KeyboardHint, Skeleton } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type QAIssue, type Task, type WorkProject } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { errorMessage, isComplete, type Answer, type TaskType } from "@/lib/onboarding";
import { formatKes } from "@/lib/projects";
import {
  blockedIssues,
  rationaleShortBy,
  reportCategories,
  secondsLeft,
  skipReasons,
  submitKey,
  timeLeft,
} from "@/lib/tasks";
import { useLoad } from "@/lib/use-load";
import { checkList } from "./work";

type Phase =
  | { k: "loading" }
  | { k: "task"; task: Task }
  | { k: "none" }
  | { k: "expired"; task: Task }
  | { k: "error"; error: ApiError };
type Save = "idle" | "saving" | "saved" | "failed";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

function storage(): Storage | null {
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

/** /c/work/[projectId]: one task at a time, keyboard first (DESIGN.md section 11). */
export function TaskWorkspace() {
  const { projectId } = useParams<{ projectId: string }>();
  const project = useLoad(() => api<WorkProject>("GET", `/v1/me/projects/${projectId}`), projectId);
  const [phase, setPhase] = useState<Phase>({ k: "loading" });
  const [done, setDone] = useState(0);
  const heading = useRef<HTMLHeadingElement>(null);

  // Reserve (or resume) a task. State changes only after the request, so the
  // first load can run from an effect.
  const fetchNext = useCallback(async () => {
    try {
      const task = await api<Task>("POST", `/v1/me/projects/${projectId}/next`, {});
      setPhase({ k: "task", task });
      requestAnimationFrame(() => heading.current?.focus());
    } catch (e) {
      const err = asError(e);
      setPhase(err.code === "no_tasks" ? { k: "none" } : { k: "error", error: err });
    }
  }, [projectId]);
  const next = useCallback(() => {
    setPhase({ k: "loading" });
    void fetchNext();
  }, [fetchNext]);

  useEffect(() => {
    // fetchNext sets state only after the awaited request (same pattern as lib/session.tsx).
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (project.status === "ready") void fetchNext();
  }, [project.status, fetchNext]);

  if (project.status === "loading") return <Skeleton className="h-96 w-full" />;
  if (project.status === "error")
    return (
      <Alert
        tone="danger"
        title={errorMessage(project.error)}
        action={
          <ButtonLink href="/c/work" variant="secondary" size="sm">
            All tasks
          </ButtonLink>
        }
      />
    );
  const p = project.data;
  // Every state keeps the project name as the page heading.
  const crumbs = (
    <div className="grid gap-4">
      <Breadcrumb items={[{ label: "Tasks", href: "/c/work" }, { label: p.name }]} />
      {phase.k !== "task" && (
        <h1 className="font-display text-xl font-medium tracking-[-0.01em] md:text-2xl">{p.name}</h1>
      )}
    </div>
  );

  if (phase.k === "loading") return <Skeleton className="h-96 w-full" />;
  if (phase.k === "error")
    return (
      <div className="grid gap-6">
        {crumbs}
        <Alert
          tone="danger"
          title={errorMessage(phase.error)}
          action={
            <Button variant="secondary" size="sm" onClick={next}>
              Try again
            </Button>
          }
        />
      </div>
    );
  if (phase.k === "none")
    return (
      <div className="grid gap-6">
        {crumbs}
        <EmptyState
          title="No more tasks for you here right now"
          action={
            <ButtonLink href="/c/work" variant="secondary">
              Other projects
            </ButtonLink>
          }
        >
          {`${done > 0 ? `You submitted ${done} in this session. ` : ""}Every item still open on this project already has enough people on it, or you have done it. New items appear as the client adds them.`}
        </EmptyState>
      </div>
    );
  if (phase.k === "expired")
    return (
      <div className="grid gap-6">
        {crumbs}
        <Alert
          tone="warning"
          title="Time ran out on that task"
          action={
            <Button size="sm" onClick={next}>
              Take the next task
            </Button>
          }
        >
          It went back to the queue so someone else can finish it. Your draft was kept with it.
        </Alert>
      </div>
    );

  return (
    <TaskView
      key={phase.task.id}
      project={p}
      task={phase.task}
      done={done}
      crumbs={crumbs}
      heading={heading}
      onFinished={(submitted) => {
        if (submitted) setDone((n) => n + 1);
        void next();
      }}
      onExpired={() => setPhase({ k: "expired", task: phase.task })}
    />
  );
}

function TaskView({
  project: p,
  task,
  done,
  crumbs,
  heading,
  onFinished,
  onExpired,
}: {
  project: WorkProject;
  task: Task;
  done: number;
  crumbs: React.ReactNode;
  heading: React.RefObject<HTMLHeadingElement | null>;
  onFinished: (submitted: boolean) => void;
  onExpired: () => void;
}) {
  const type = p.task_type as TaskType;
  const toast = useToast();
  const [answer, setAnswer] = useState<Answer>(() => (task.draft as Answer | null) ?? {});
  const [save, setSave] = useState<Save>("idle");
  const [now, setNow] = useState(() => Date.now());
  const [issues, setIssues] = useState<QAIssue[]>([]);
  const [error, setError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  const [dialog, setDialog] = useState<"skip" | "report" | "keys" | "help" | null>(null);
  const started = useRef<number | null>(null);
  const pasted = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const content = (task.content ?? {}) as Record<string, unknown>;

  useEffect(() => {
    started.current = performance.now();
  }, []);
  const elapsed = () =>
    task.time_on_task_ms + (started.current === null ? 0 : Math.round(performance.now() - started.current));

  // The clock: one tick a second; expiry hands back to the parent.
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, []);
  const left = secondsLeft(task.expires_at, now);
  useEffect(() => {
    if (left === 0) onExpired();
  }, [left, onExpired]);
  const lowTime = left > 0 && left <= 300;

  // Autosave 1.5 s after the last change.
  const persist = useCallback(
    (a: Answer) => {
      clearTimeout(timer.current);
      setSave("saving");
      timer.current = setTimeout(() => {
        api("PUT", `/v1/me/tasks/${task.id}/draft`, { answer: a, time_on_task_ms: elapsed() }).then(
          () => setSave("saved"),
          (e) => (asError(e).code === "reservation_closed" ? onExpired() : setSave("failed")),
        );
      }, 1500);
    },
    // elapsed reads refs only
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [task.id, onExpired],
  );
  useEffect(() => () => clearTimeout(timer.current), []);

  const onChange = useCallback(
    (a: Answer) => {
      setAnswer(a);
      setIssues([]);
      persist(a);
    },
    [persist],
  );

  async function submit() {
    setError(null);
    if (!isComplete(type, p.config, answer)) {
      setError(new ApiError(422, "invalid_input", "answer", "Answer every part of the task first."));
      return;
    }
    setBusy(true);
    clearTimeout(timer.current);
    try {
      const res = await api<{ warnings: QAIssue[] }>(
        "POST",
        `/v1/me/tasks/${task.id}/submit`,
        { answer, signals: { time_on_task_ms: elapsed(), pasted_chars: pasted.current } },
        { "Idempotency-Key": submitKey(task.id, storage()) },
      );
      toast({
        tone: "success",
        title: "Submitted",
        body: res.warnings.length
          ? `Sent with a note for the reviewer: ${res.warnings.map((w) => w.message).join(" ")}`
          : undefined,
      });
      onFinished(true);
    } catch (e) {
      const err = asError(e);
      if (err.code === "already_submitted") return onFinished(true);
      if (err.code === "reservation_closed") return onExpired();
      if (err.code === "qa_blocked") setIssues(blockedIssues(err.body));
      else setError(err);
    } finally {
      setBusy(false);
    }
  }

  // Ctrl/Cmd + Enter submits; "?" lists the shortcuts (not while typing).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (dialog) return;
      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        void submit();
      } else if (e.key === "?" && !(e.target as HTMLElement).closest("input,textarea,select")) {
        e.preventDefault();
        setDialog("keys");
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  const short = rationaleShortBy(p, answer);
  const fieldErrors = Object.fromEntries(issues.filter((i) => i.field).map((i) => [i.field!, i.message]));

  return (
    <div className="grid gap-6 pb-44 md:pb-24">
      <div className="grid gap-4">
        {crumbs}
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1
            ref={heading}
            tabIndex={-1}
            className="font-display text-xl font-medium tracking-[-0.01em] outline-none md:text-2xl"
          >
            {p.name}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
            <p
              className={cn(
                "rounded-control border px-2.5 py-1.5 font-mono text-sm tabular",
                lowTime ? "border-warning-fg text-warning-fg" : "border-border text-fg-muted",
              )}
            >
              <span className="sr-only">Time left: </span>
              {timeLeft(task.expires_at, now)}
            </p>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDialog("help")}
              icon={<BookOpen aria-hidden="true" className="size-4" />}
            >
              Instructions
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDialog("skip")}
              icon={<SkipForward aria-hidden="true" className="size-4" />}
            >
              Skip
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDialog("report")}
              icon={<Flag aria-hidden="true" className="size-4" />}
            >
              Report
            </Button>
          </div>
        </div>
        <p className="text-sm text-fg-muted">
          {formatKes(p.pay_per_task_cents, p.currency)} a task · {p.my_submitted + done} submitted on this
          project
          {done > 0 ? ` (${done} this session)` : ""}
        </p>
        <p className="sr-only" aria-live="polite">
          {lowTime ? "Five minutes or less left on this task." : ""}
        </p>
      </div>

      {error && <Alert tone="danger" title={errorMessage(error)} />}
      {issues.length > 0 && (
        <Alert tone="danger" title={errorMessage(new ApiError(422, "qa_blocked"))}>
          <ul className="list-disc pl-5">
            {issues.map((i) => (
              <li key={i.code}>{i.message}</li>
            ))}
          </ul>
        </Alert>
      )}

      <section aria-label="Task" className="grid gap-8">
        <QuestionContent type={type} content={content} />
        <AnswerInput
          type={type}
          config={p.config}
          content={content}
          value={answer}
          onChange={onChange}
          onPaste={(n) => (pasted.current += n)}
          fieldErrors={fieldErrors}
          rationaleHint={
            short > 0
              ? `One or two sentences on what decided it. At least ${p.min_rationale_chars} characters (${short} to go).`
              : undefined
          }
        />
      </section>

      <div className="fixed inset-x-0 bottom-16 z-30 border-t border-border bg-surface/95 backdrop-blur md:bottom-0">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center gap-3 px-4 py-3 md:px-8">
          <p className="text-sm text-fg-muted" aria-live="polite">
            {save === "saving"
              ? "Saving…"
              : save === "saved"
                ? "Draft saved"
                : save === "failed"
                  ? "Not saved. Check your connection."
                  : ""}
          </p>
          <button
            type="button"
            onClick={() => setDialog("keys")}
            className="ml-auto hidden items-center gap-1.5 text-sm text-fg-muted underline-offset-4 hover:underline sm:inline-flex"
          >
            <Keyboard aria-hidden="true" className="size-4" /> Shortcuts
          </button>
          <KeyboardHint keys={["Ctrl", "Enter"]} label="submit" className="hidden sm:inline-flex" />
          <Button onClick={submit} loading={busy} className="ml-auto sm:ml-0">
            Submit
          </Button>
        </div>
      </div>

      <Drawer open={dialog === "help"} onClose={() => setDialog(null)} title="Instructions">
        <p className="whitespace-pre-wrap">{p.instructions_md || "No instructions yet."}</p>
        {checkList(p).length > 0 && (
          <>
            <h3 className="mt-6 font-display font-semibold">Checked before you submit</h3>
            <ul className="mt-2 grid list-disc gap-1.5 pl-5 text-sm text-fg-muted">
              {checkList(p).map((c) => (
                <li key={c}>{c}</li>
              ))}
            </ul>
          </>
        )}
      </Drawer>

      <Dialog open={dialog === "keys"} onClose={() => setDialog(null)} title="Keyboard shortcuts">
        <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-3 text-sm">
          {type === "classification" && (
            <>
              <dt>
                <KeyboardHint keys={["1", "…", "9"]} label="" />
              </dt>
              <dd>Choose a label</dd>
            </>
          )}
          <dt>
            <KeyboardHint keys={["Ctrl", "Enter"]} label="" />
          </dt>
          <dd>Submit</dd>
          <dt>
            <KeyboardHint keys={["?"]} label="" />
          </dt>
          <dd>Show this list</dd>
          <dt>
            <KeyboardHint keys={["Esc"]} label="" />
          </dt>
          <dd>Close a dialog</dd>
        </dl>
      </Dialog>

      <SkipDialog
        open={dialog === "skip"}
        taskId={task.id}
        onClose={() => setDialog(null)}
        onDone={() => onFinished(false)}
        onExpired={onExpired}
      />
      <ReportDialog
        open={dialog === "report"}
        taskId={task.id}
        onClose={() => setDialog(null)}
        onDone={() => onFinished(false)}
      />
    </div>
  );
}

function SkipDialog({
  open,
  taskId,
  onClose,
  onDone,
  onExpired,
}: {
  open: boolean;
  taskId: string;
  onClose: () => void;
  onDone: () => void;
  onExpired: () => void;
}) {
  const [reason, setReason] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  async function skip() {
    if (!reason) return setError(new ApiError(422, "invalid_input", "reason", "Choose a reason."));
    if (reason === "other" && !note.trim())
      return setError(new ApiError(422, "invalid_input", "note", "Say what the problem is."));
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/me/tasks/${taskId}/skip`, { reason, note: note.trim() });
      onClose();
      onDone();
    } catch (e) {
      const err = asError(e);
      if (err.code === "reservation_closed") {
        onClose();
        return onExpired();
      }
      setError(err);
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Skip this task?"
      description="You will not see it again. Skipping is fine and does not count against you."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Keep working
          </Button>
          <Button onClick={skip} loading={busy}>
            Skip task
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} />}
        <ChoiceGroup
          legend="Why are you skipping?"
          error={error?.field === "reason" ? errorMessage(error) : undefined}
        >
          {skipReasons.map((r) => (
            <Radio
              key={r.value}
              name="skip-reason"
              value={r.value}
              checked={reason === r.value}
              onChange={() => setReason(r.value)}
              label={r.label}
            />
          ))}
        </ChoiceGroup>
        <Field
          label="Note"
          hint={reason === "other" ? "Required for “another reason”." : "Optional."}
          error={error?.field === "note" ? errorMessage(error) : undefined}
          required={reason === "other"}
        >
          <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={1000} />
        </Field>
      </div>
    </Dialog>
  );
}

function ReportDialog({
  open,
  taskId,
  onClose,
  onDone,
}: {
  open: boolean;
  taskId: string;
  onClose: () => void;
  onDone: () => void;
}) {
  const toast = useToast();
  const [category, setCategory] = useState("");
  const [note, setNote] = useState("");
  const [error, setError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  async function report() {
    if (!category) return setError(new ApiError(422, "invalid_input", "category", "Choose what is wrong."));
    if (category === "other" && !note.trim())
      return setError(new ApiError(422, "invalid_input", "note", "Say what the problem is."));
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/me/tasks/${taskId}/report`, { category, note: note.trim() });
      toast({
        tone: "success",
        title: "Reported",
        body: "Thank you. The MezAI team will look at it, and you will not see this item again.",
      });
      onClose();
      onDone();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Report this content"
      description="The task leaves your queue now, and nobody else gets it until the MezAI team has looked at it."
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="danger" onClick={report} loading={busy}>
            Report
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} />}
        <ChoiceGroup
          legend="What is wrong?"
          error={error?.field === "category" ? errorMessage(error) : undefined}
        >
          {reportCategories.map((c) => (
            <Radio
              key={c.value}
              name="report-category"
              value={c.value}
              checked={category === c.value}
              onChange={() => setCategory(c.value)}
              label={c.label}
            />
          ))}
        </ChoiceGroup>
        <Field
          label="Note"
          hint={
            category === "other"
              ? "Required for “something else”."
              : "Optional. Do not copy the content here."
          }
          error={error?.field === "note" ? errorMessage(error) : undefined}
          required={category === "other"}
        >
          <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={1000} />
        </Field>
      </div>
    </Dialog>
  );
}
