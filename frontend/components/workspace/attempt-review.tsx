"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { AnswerSummary, QuestionContent } from "@/components/qualification/content";
import { Button } from "@/components/ui/button";
import { Alert, Breadcrumb, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type AttemptReview as Review } from "@/lib/api/client";
import { errorMessage, type Answer, type TaskType } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { pct, shortDate } from "./contributors";
import { PageTitle } from "./workspace";

export function AttemptReview() {
  const { id } = useParams<{ id: string }>();
  const r = useLoad(() => api<Review>("GET", `/v1/qualification/attempts/${id}`), id);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState<"approve" | "reject" | null>(null);
  const [error, setError] = useState<ApiError | null>(null);
  const toast = useToast();

  if (r.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (r.status === "error") return <Alert tone="danger" title={errorMessage(r.error)} />;
  const a = r.data;
  const type = a.task_type as TaskType;
  const open = a.status === "scored";

  async function decide(decision: "approve" | "reject") {
    if (decision === "reject" && !note.trim()) {
      setError(
        new ApiError(422, "invalid_input", "note", "Write a note so the contributor knows what to improve."),
      );
      return;
    }
    setBusy(decision);
    setError(null);
    try {
      const next = await api<Review>("POST", `/v1/qualification/attempts/${a.id}/decision`, {
        decision,
        note: note.trim(),
      });
      if (r.status === "ready") r.set(next);
      toast({ tone: "success", title: decision === "approve" ? "Attempt approved" : "Attempt rejected" });
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="grid gap-6">
      <Breadcrumb
        items={[
          { label: "Contributors", href: "/w/contributors" },
          { label: `${a.contributor.public_code} · ${a.test_title}` },
        ]}
      />
      <PageTitle title={a.test_title}>
        <Link href={`/w/contributors/${a.contributor.id}`} className="underline underline-offset-4">
          <span className="font-mono">{a.contributor.public_code}</span> {a.contributor.display_name}
        </Link>{" "}
        · submitted {shortDate(a.submitted_at)}
      </PageTitle>

      <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
        <ol className="grid content-start gap-4">
          {a.questions.map((q) => (
            <li key={q.id}>
              <Card className="grid gap-4 p-4">
                <div className="flex items-center justify-between gap-3">
                  <h2 className="font-display font-semibold">Question {q.position}</h2>
                  {q.correct == null ? null : (
                    <StatusPill status={q.correct ? "success" : "danger"}>
                      {q.correct ? "Matches" : "Does not match"}
                    </StatusPill>
                  )}
                </div>
                <QuestionContent type={type} content={q.content as Record<string, unknown>} />
                <dl className="grid gap-3 text-sm sm:grid-cols-2">
                  <div className="grid gap-1">
                    <dt className="font-mono text-xs text-fg-muted uppercase">Contributor</dt>
                    <dd>
                      <AnswerSummary type={type} answer={q.answer as Answer | null} />
                    </dd>
                  </div>
                  <div className="grid gap-1">
                    <dt className="font-mono text-xs text-fg-muted uppercase">Expected</dt>
                    <dd>
                      <AnswerSummary type={type} answer={q.expected as Answer} />
                    </dd>
                  </div>
                </dl>
              </Card>
            </li>
          ))}
        </ol>

        <aside className="grid content-start gap-4 lg:sticky lg:top-6">
          <Card className="grid gap-3 p-5">
            <p className="text-sm text-fg-muted">Score / pass mark</p>
            <p className="font-mono text-3xl tabular">
              {pct(a.score)} <span className="text-lg text-fg-muted">/ {pct(a.pass_threshold)}</span>
            </p>
            <StatusPill status={a.passed ? "success" : "warning"}>
              {a.passed ? "Passed" : "Below pass mark"}
            </StatusPill>
          </Card>
          {open ? (
            <Card className="grid gap-4 p-5">
              <h2 className="font-display font-semibold">Decision</h2>
              {error && !error.field && <Alert tone="danger" title={errorMessage(error)} />}
              <Field
                label="Note to the contributor"
                hint="Required to reject. The contributor sees it."
                error={error?.field === "note" ? errorMessage(error) : undefined}
              >
                <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={4} maxLength={1000} />
              </Field>
              <div className="flex flex-wrap gap-2">
                <Button
                  onClick={() => decide("approve")}
                  loading={busy === "approve"}
                  disabled={!a.passed || busy !== null}
                >
                  Approve
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => decide("reject")}
                  loading={busy === "reject"}
                  disabled={busy !== null}
                >
                  Reject
                </Button>
              </div>
              {!a.passed && (
                <p className="text-sm text-fg-muted">Below the pass mark, so it can only be rejected.</p>
              )}
            </Card>
          ) : (
            <Alert
              tone={a.status === "approved" ? "success" : "warning"}
              title={
                a.status === "approved"
                  ? "Approved"
                  : a.status === "rejected"
                    ? "Rejected"
                    : "Not submitted yet"
              }
            >
              {a.decided_at && <>Decided {shortDate(a.decided_at)}. </>}
              {a.decision_note}
            </Alert>
          )}
        </aside>
      </div>
    </div>
  );
}
