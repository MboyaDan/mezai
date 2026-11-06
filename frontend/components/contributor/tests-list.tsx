"use client";

import { ButtonLink } from "@/components/ui/button";
import { Alert, Card, EmptyState, Skeleton, StatusPill, Tag } from "@/components/ui/display";
import { TextLink } from "@/components/ui/text-link";
import { labelFor } from "@/content/profile";
import { api, type MyTest } from "@/lib/api/client";
import { errorMessage, testStateLabel } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { useProfile } from "./portal";

const taskLabel = {
  classification: "Text classification",
  pairwise: "Response comparison",
  translation_review: "Translation review",
};
const tone = {
  available: "neutral",
  in_progress: "info",
  awaiting_review: "info",
  qualified: "success",
  not_passed: "warning",
} as const;

export const skillLabel = (t: Pick<MyTest, "skill_kind" | "skill_value">) =>
  t.skill_kind === "task_type"
    ? (taskLabel[t.skill_value as keyof typeof taskLabel] ?? t.skill_value)
    : labelFor(t.skill_value);

const date = (iso: string) => new Date(iso).toLocaleDateString("en-KE", { day: "numeric", month: "long" });

/** Lime is for the one next action (DESIGN.md: lime at most about 5 percent of a screen). */
function Action({ t, primary }: { t: MyTest; primary: boolean }) {
  const variant = primary ? "primary" : "secondary";
  const cls = "justify-self-start";
  if (t.state === "in_progress" && t.can_start)
    return (
      <ButtonLink href={`/c/qualify/${t.id}`} variant={variant} className={cls} arrow>
        Continue
      </ButtonLink>
    );
  if (t.can_start)
    return (
      <ButtonLink
        href={`/c/qualify/${t.id}`}
        variant={t.state === "not_passed" ? "secondary" : variant}
        className={cls}
        arrow
      >
        {t.state === "not_passed" ? "Retake" : "Start"}
      </ButtonLink>
    );
  if (t.state === "not_passed" && t.retake_after)
    return <span className="text-sm text-fg-muted">Retake from {date(t.retake_after)}</span>;
  return null;
}

export function TestsList() {
  const { profile } = useProfile();
  const tests = useLoad(() =>
    api<{ tests: MyTest[] }>("GET", "/v1/me/qualification/tests").then((r) => r.tests),
  );

  return (
    <div className="grid gap-8">
      <header className="grid max-w-2xl gap-2">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          Qualification
        </h1>
        <p className="text-fg-muted">
          Each test checks one skill with a few items whose answers we already know. It is scored as soon as
          you submit; then a person on the MezAI team reviews it. Take the tests that match your languages and
          strengths.
        </p>
      </header>

      {profile.status === "applied" && (
        <Alert
          tone="info"
          title="Complete your profile first"
          action={
            <ButtonLink href="/c/profile" variant="secondary" size="sm">
              Go to profile
            </ButtonLink>
          }
        >
          Tests open once your languages, domains and M-Pesa number are in.
        </Alert>
      )}

      {tests.status === "loading" && <Skeleton className="h-40 w-full" />}
      {tests.status === "error" && <Alert tone="danger" title={errorMessage(tests.error)} />}
      {tests.status === "ready" &&
        (tests.data.length === 0 ? (
          <EmptyState title="No tests are open right now">
            New tests open as projects start. Check back soon; we will also email you.
          </EmptyState>
        ) : (
          <ul className="grid gap-4">
            {tests.data.map((t, i) => (
              <li key={t.id}>
                <Card className="grid gap-4 p-5 md:grid-cols-[1fr_auto] md:items-center">
                  <div className="grid gap-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <h2 className="font-display text-lg font-semibold">{t.title}</h2>
                      <StatusPill status={tone[t.state]}>{testStateLabel[t.state]}</StatusPill>
                    </div>
                    <p className="flex flex-wrap items-center gap-2 text-sm text-fg-muted">
                      <Tag>{skillLabel(t)}</Tag>
                      {t.skill_kind !== "task_type" && (
                        <>
                          <span>{taskLabel[t.task_type]}</span>
                          <span aria-hidden="true">·</span>
                        </>
                      )}
                      <span>{t.question_count} questions</span>
                      <span aria-hidden="true">·</span>
                      <span>
                        Pass mark <span className="font-mono">{Math.round(t.pass_threshold * 100)}%</span>
                      </span>
                      {t.score != null && t.state !== "in_progress" && (
                        <>
                          <span aria-hidden="true">·</span>
                          <span>
                            Your score <span className="font-mono">{Math.round(t.score * 100)}%</span>
                          </span>
                        </>
                      )}
                    </p>
                    {t.state === "not_passed" && t.decision_note && (
                      <p className="border-l-2 border-warning pl-3 text-sm">
                        <span className="font-medium">Reviewer&apos;s note:</span> {t.decision_note}
                      </p>
                    )}
                  </div>
                  <Action t={t} primary={i === tests.data.findIndex((x) => x.can_start)} />
                </Card>
              </li>
            ))}
          </ul>
        ))}
      <p className="text-sm text-fg-muted">
        Need help? Read the <TextLink href="/legal/contributor-terms">contributor terms</TextLink> for how
        tests and pay work.
      </p>
    </div>
  );
}
