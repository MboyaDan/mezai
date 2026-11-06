"use client";

import { useParams } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { AnswerInput } from "@/components/qualification/answer-input";
import { QuestionContent } from "@/components/qualification/content";
import { Button, ButtonLink } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Breadcrumb, Card, KeyboardHint, Skeleton, StatusPill } from "@/components/ui/display";
import { ProgressBar } from "@/components/ui/data";
import { ApiError, api, type Attempt, type MyTest } from "@/lib/api/client";
import { errorMessage, isComplete, type Answer, type TaskType } from "@/lib/onboarding";
import { cn } from "@/lib/cn";
import { useLoad } from "@/lib/use-load";
import { useProfile } from "./portal";
import { skillLabel } from "./tests-list";

type Answers = Record<string, Answer>;
type Save = "idle" | "saving" | "saved" | "failed";

export function TakeTest() {
  const { testId } = useParams<{ testId: string }>();
  const { reload: reloadProfile } = useProfile();
  const tests = useLoad(
    () => api<{ tests: MyTest[] }>("GET", "/v1/me/qualification/tests").then((r) => r.tests),
    testId,
  );
  const [attempt, setAttempt] = useState<Attempt | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  const test = tests.status === "ready" ? tests.data.find((t) => t.id === testId) : undefined;

  // Resume an attempt in progress without a click.
  useEffect(() => {
    if (test?.state === "in_progress" && test.attempt_id && !attempt)
      api<Attempt>("GET", `/v1/me/qualification/attempts/${test.attempt_id}`).then(setAttempt, (e) =>
        setError(e),
      );
  }, [test, attempt]);

  async function begin() {
    setBusy(true);
    setError(null);
    try {
      setAttempt(await api<Attempt>("POST", `/v1/me/qualification/tests/${testId}/attempts`, {}));
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  const crumbs = (
    <Breadcrumb items={[{ label: "Qualification", href: "/c/qualify" }, { label: test?.title ?? "Test" }]} />
  );

  if (tests.status === "loading" || (test?.state === "in_progress" && !attempt && !error))
    return <Skeleton className="h-80 w-full" />;
  if (tests.status === "error") return <Alert tone="danger" title={errorMessage(tests.error)} />;
  if (!test)
    return (
      <div className="grid gap-6">
        {crumbs}
        <Alert
          tone="warning"
          title="This test is not open"
          action={
            <ButtonLink href="/c/qualify" variant="secondary" size="sm">
              All tests
            </ButtonLink>
          }
        />
      </div>
    );

  if (attempt && attempt.status === "in_progress")
    return (
      <AttemptView
        attempt={attempt}
        crumbs={crumbs}
        onSubmitted={(a) => {
          setAttempt(a);
          reloadProfile();
        }}
      />
    );
  if (attempt) return <Result attempt={attempt} crumbs={crumbs} />;

  return (
    <div className="grid max-w-3xl gap-8">
      {crumbs}
      <header className="grid gap-3">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          {test.title}
        </h1>
        <p className="text-fg-muted">
          {skillLabel(test)} · {test.question_count} questions · pass mark{" "}
          <span className="font-mono">{Math.round(test.pass_threshold * 100)}%</span>
        </p>
      </header>
      {test.instructions_md && (
        <Card className="p-5">
          <h2 className="font-display text-lg font-semibold">Instructions</h2>
          <p className="mt-2 whitespace-pre-wrap">{test.instructions_md}</p>
        </Card>
      )}
      <ul className="grid gap-2 text-sm text-fg-muted">
        <li>Your answers save as you go. You can leave and come back.</li>
        <li>There is no timer. Read each item carefully.</li>
        <li>After you submit, a person reviews your attempt. You cannot change answers after submitting.</li>
      </ul>
      {error && <Alert tone="danger" title={errorMessage(error)} />}
      <div>
        {test.can_start ? (
          <Button onClick={begin} loading={busy} arrow>
            Begin the test
          </Button>
        ) : (
          <Alert
            tone="info"
            title={
              test.state === "awaiting_review"
                ? "Waiting for review"
                : test.state === "qualified"
                  ? "You have qualified on this test"
                  : "This test is closed for you right now"
            }
          />
        )}
      </div>
    </div>
  );
}

function AttemptView({
  attempt,
  crumbs,
  onSubmitted,
}: {
  attempt: Attempt;
  crumbs: React.ReactNode;
  onSubmitted: (a: Attempt) => void;
}) {
  const type = attempt.task_type as TaskType;
  const [answers, setAnswers] = useState<Answers>(() => attempt.answers as Answers);
  const [i, setI] = useState(() => {
    const first = attempt.questions.findIndex(
      (q) => !isComplete(type, attempt.config, (attempt.answers as Answers)[q.id]),
    );
    return first === -1 ? attempt.questions.length - 1 : first;
  });
  const [save, setSave] = useState<Save>("idle");
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const heading = useRef<HTMLHeadingElement>(null);
  const q = attempt.questions[i];
  const total = attempt.questions.length;
  const done = attempt.questions.filter((x) => isComplete(type, attempt.config, answers[x.id])).length;
  const last = i === total - 1;

  // Autosave 800 ms after the last change.
  const persist = useCallback(
    (next: Answers) => {
      clearTimeout(timer.current);
      setSave("saving");
      timer.current = setTimeout(() => {
        api("PUT", `/v1/me/qualification/attempts/${attempt.id}/answers`, { answers: next }).then(
          () => setSave("saved"),
          () => setSave("failed"),
        );
      }, 800);
    },
    [attempt.id],
  );
  useEffect(() => () => clearTimeout(timer.current), []);

  const onChange = useCallback(
    (a: Answer) => {
      setAnswers((prev) => {
        const next = { ...prev, [q.id]: a };
        persist(next);
        return next;
      });
    },
    [q.id, persist],
  );

  function go(to: number) {
    setI(to);
    requestAnimationFrame(() => heading.current?.focus());
  }

  // Ctrl/Cmd + Enter: next question, or open the submit check on the last one.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Enter" || !(e.metaKey || e.ctrlKey)) return;
      e.preventDefault();
      if (last) setConfirm(true);
      else go(i + 1);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  async function submit() {
    setBusy(true);
    setError(null);
    clearTimeout(timer.current);
    try {
      onSubmitted(
        await api<Attempt>("POST", `/v1/me/qualification/attempts/${attempt.id}/submit`, { answers }),
      );
    } catch (e) {
      const a = e instanceof ApiError ? e : new ApiError(0, "network");
      setError(a);
      setConfirm(false);
      const bad = a.field?.startsWith("answers.")
        ? attempt.questions.findIndex((x) => `answers.${x.id}` === a.field)
        : -1;
      if (bad >= 0) go(bad);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-8">
      <div className="grid gap-4">
        {crumbs}
        <div className="flex flex-wrap items-end justify-between gap-4">
          <h1
            ref={heading}
            tabIndex={-1}
            className="font-display text-2xl font-medium tracking-[-0.02em] outline-none"
          >
            Question {i + 1} <span className="text-fg-muted">of {total}</span>
          </h1>
          <p className="text-sm text-fg-muted" aria-live="polite">
            {save === "saving"
              ? "Saving…"
              : save === "saved"
                ? "All answers saved"
                : save === "failed"
                  ? "Not saved. Check your connection."
                  : ""}
          </p>
        </div>
        <ProgressBar label="Answered" value={done} max={total} valueText={`${done} of ${total}`} />
      </div>

      {error && <Alert tone="danger" title={errorMessage(error)} />}

      <section aria-label={`Question ${i + 1}`} className="grid gap-8">
        <QuestionContent type={type} content={q.content as Record<string, unknown>} />
        <AnswerInput
          key={q.id}
          type={type}
          config={attempt.config}
          content={q.content as Record<string, unknown>}
          value={answers[q.id]}
          onChange={onChange}
        />
      </section>

      <nav aria-label="Questions" className="flex flex-wrap items-center gap-3 border-t border-border pt-6">
        <Button
          variant="secondary"
          disabled={i === 0}
          onClick={() => go(i - 1)}
          icon={<ArrowLeft aria-hidden="true" className="size-4" />}
        >
          Previous
        </Button>
        {last ? (
          <Button onClick={() => setConfirm(true)}>Review and submit</Button>
        ) : (
          <Button onClick={() => go(i + 1)} icon={<ArrowRight aria-hidden="true" className="size-4" />}>
            Next
          </Button>
        )}
        <KeyboardHint
          keys={["Ctrl", "Enter"]}
          label={last ? "submit" : "next"}
          className="ml-auto hidden sm:inline-flex"
        />
        <ol className="flex w-full flex-wrap gap-1.5 pt-2" aria-label="Jump to question">
          {attempt.questions.map((x, j) => {
            const ok = isComplete(type, attempt.config, answers[x.id]);
            return (
              <li key={x.id}>
                <button
                  type="button"
                  onClick={() => go(j)}
                  aria-current={j === i ? "step" : undefined}
                  aria-label={`Question ${j + 1}${ok ? ", answered" : ", not answered"}`}
                  className={cn(
                    "grid size-9 place-items-center rounded-control border font-mono text-xs",
                    j === i ? "border-fg" : "border-border",
                    ok ? "bg-fg text-bg" : "bg-surface text-fg",
                  )}
                >
                  {j + 1}
                </button>
              </li>
            );
          })}
        </ol>
      </nav>

      <Dialog
        open={confirm}
        onClose={() => setConfirm(false)}
        title="Submit your answers?"
        description={
          done === total
            ? "You cannot change them after submitting."
            : `${total - done} of ${total} questions still need an answer.`
        }
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(false)}>
              Keep working
            </Button>
            <Button onClick={submit} loading={busy} disabled={done < total}>
              Submit
            </Button>
          </>
        }
      />
    </div>
  );
}

function Result({ attempt, crumbs }: { attempt: Attempt; crumbs: React.ReactNode }) {
  const pct = attempt.score != null ? Math.round(attempt.score * 100) : null;
  return (
    <div className="grid max-w-2xl gap-8">
      {crumbs}
      <header className="grid gap-3">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          Submitted
        </h1>
        <p className="text-fg-muted">{attempt.title}</p>
      </header>
      <Card className="grid gap-4 p-6 sm:grid-cols-[auto_1fr] sm:items-center sm:gap-8">
        {pct != null && (
          <p className="font-mono text-5xl font-medium tabular">
            {pct}
            <span className="text-2xl text-fg-muted">%</span>
          </p>
        )}
        <div className="grid gap-2">
          <StatusPill status={attempt.passed ? "success" : "warning"}>
            {attempt.passed ? "Above the pass mark" : "Below the pass mark"}
          </StatusPill>
          <p className="text-sm text-fg-muted">
            {attempt.status === "scored"
              ? attempt.passed
                ? "A person on the MezAI team reviews every passed test. We will email you when it is done."
                : "A reviewer will add a note on what to look at, and you can retake the test after a week."
              : (attempt.decision_note ?? "")}
          </p>
        </div>
      </Card>
      <div className="flex flex-wrap gap-3">
        <ButtonLink href="/c" variant="secondary">
          Back to dashboard
        </ButtonLink>
        <ButtonLink href="/c/qualify" variant="ghost">
          Other tests
        </ButtonLink>
      </div>
    </div>
  );
}
