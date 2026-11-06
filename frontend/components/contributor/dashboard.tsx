"use client";

import { Check } from "lucide-react";
import type { ReactNode } from "react";
import { ButtonLink } from "@/components/ui/button";
import { Alert, Card, Skeleton, StatusPill, Tag } from "@/components/ui/display";
import { api, type MyTest } from "@/lib/api/client";
import { currentStep, errorMessage, fieldLabels, type Step } from "@/lib/onboarding";
import { labelFor } from "@/content/profile";
import { cn } from "@/lib/cn";
import { useLoad } from "@/lib/use-load";
import { useProfile } from "./portal";
import { StatusBadge } from "./status";

type Row = { id: string; title: string; body: ReactNode; action?: ReactNode };

const order: Step[] = ["consent", "profile", "qualify", "review", "active"];

export function Dashboard() {
  const { profile } = useProfile();
  const tests = useLoad(() =>
    api<{ tests: MyTest[] }>("GET", "/v1/me/qualification/tests").then((r) => r.tests),
  );
  const first = profile.display_name.split(/\s+/)[0];

  if (tests.status === "loading") return <Skeleton className="h-72 w-full" />;
  if (tests.status === "error") return <Alert tone="danger" title={errorMessage(tests.error)} />;

  const step = currentStep(profile, tests.data);
  const waiting = tests.data.filter((t) => t.state === "awaiting_review");
  const rejected = tests.data.filter((t) => t.state === "not_passed");

  const rows: Row[] = [
    {
      id: "consent",
      title: "Create your account",
      body: "Email confirmed, privacy notice and contributor terms accepted.",
    },
    {
      id: "profile",
      title: "Complete your profile",
      body:
        profile.missing.length > 0 ? (
          <>Still needed: {profile.missing.map((f) => fieldLabels[f] ?? f).join(", ")}.</>
        ) : (
          "Languages, domains, availability and your M-Pesa number."
        ),
      action: (
        <ButtonLink href="/c/profile" arrow>
          Complete profile
        </ButtonLink>
      ),
    },
    {
      id: "qualify",
      title: "Pass a qualification test",
      body: (
        <>
          A short test for one skill, built from items with known answers. It is scored automatically.
          {rejected.length > 0 &&
            " One of your attempts was not passed; the reviewer's note is on the Qualification page."}
        </>
      ),
      action: (
        <ButtonLink href="/c/qualify" arrow>
          Choose a test
        </ButtonLink>
      ),
    },
    {
      id: "review",
      title: "Review by the MezAI team",
      body:
        waiting.length > 0
          ? `Your ${waiting.map((t) => t.title).join(", ")} attempt is waiting for a person to review it.`
          : "A person checks every passed test before you can take paid tasks.",
    },
  ];
  const at = order.indexOf(step);

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
        </p>
      </header>

      {step === "paused" && (
        <Alert tone="warning" title="Your account is paused">
          You cannot take tests or tasks while paused. The MezAI team will contact you by email about next
          steps.
        </Alert>
      )}
      {step === "removed" && (
        <Alert tone="danger" title="This account has been closed">
          Your history and any earnings records are kept. Contact the MezAI team if you think this is a
          mistake.
        </Alert>
      )}

      <div className="grid gap-10 lg:grid-cols-12">
        <section aria-labelledby="steps" className="lg:col-span-7">
          <h2 id="steps" className="mb-5 font-display text-lg font-semibold">
            {step === "active" ? "You are active" : "Getting started"}
          </h2>
          <ol className="grid">
            {rows.map((r, i) => {
              const state = step === "active" || i < at ? "done" : i === at ? "current" : "next";
              return (
                <li key={r.id} className="relative grid grid-cols-[2rem_1fr] gap-4 pb-8 last:pb-0">
                  {i < rows.length - 1 && (
                    <span aria-hidden="true" className="absolute top-8 bottom-0 left-4 w-px bg-border" />
                  )}
                  <span
                    aria-hidden="true"
                    className={cn(
                      "relative grid size-8 place-items-center rounded-full border font-mono text-xs",
                      state === "done" && "border-fg bg-fg text-bg",
                      state === "current" && "border-fg bg-surface text-fg",
                      state === "next" && "border-border bg-surface text-fg-muted",
                    )}
                  >
                    {state === "done" ? <Check className="size-4" /> : i + 1}
                  </span>
                  <div className="grid gap-1.5 pt-1">
                    <h3 className={cn("font-medium", state === "next" && "text-fg-muted")}>
                      {r.title}
                      <span className="sr-only">
                        {state === "done"
                          ? " (done)"
                          : state === "current"
                            ? " (current step)"
                            : " (not yet)"}
                      </span>
                    </h3>
                    <p className="text-sm text-fg-muted">{r.body}</p>
                    {state === "current" && r.action && <div className="mt-2">{r.action}</div>}
                  </div>
                </li>
              );
            })}
          </ol>
        </section>

        <aside className="grid content-start gap-6 lg:col-span-5">
          {profile.qualifications.length > 0 && (
            <Card className="p-5">
              <h2 className="font-display text-lg font-semibold">Qualified skills</h2>
              <ul className="mt-3 flex flex-wrap gap-2">
                {profile.qualifications.map((q) => (
                  <li key={q.test_id}>
                    <Tag>{q.title}</Tag>
                  </li>
                ))}
              </ul>
            </Card>
          )}
          <Card className="p-5">
            <h2 className="font-display text-lg font-semibold">What happens next</h2>
            <p className="mt-2 text-sm text-fg-muted">
              {step === "active"
                ? "Projects that match your qualified skills will be listed here as they open, with the pay per task and how many tasks are left. Paid tasks are not open yet during the pilot set-up."
                : "Once you are approved, projects that match your skills appear here with the pay per task. We pay approved work through M-Pesa."}
            </p>
          </Card>
          {waiting.length > 0 && (
            <Card className="flex items-center justify-between gap-4 p-5">
              <span className="text-sm">
                {waiting.length === 1 ? "1 test" : `${waiting.length} tests`} waiting for review
              </span>
              <StatusPill status="info">In review</StatusPill>
            </Card>
          )}
        </aside>
      </div>
    </div>
  );
}
