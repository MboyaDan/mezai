"use client";

import { Accordion, Alert, Card, EmptyState, Skeleton, Tag } from "@/components/ui/display";
import { ButtonLink } from "@/components/ui/button";
import { faqs, workedExamples, type WorkedExample } from "@/content/training";
import { api, type MyQuality } from "@/lib/api/client";
import { score } from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import { taskTypeLabel } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";

type TrainingProject = {
  id: string;
  name: string;
  task_type: keyof typeof taskTypeLabel;
  instructions_md: string;
  guidelines: { title: string; body_md: string }[];
};

const h2 = "font-display text-xl font-semibold";

/** /c/training: project guidelines, how quality is scored, worked examples and FAQs. */
export function TrainingPage() {
  const projects = useLoad(() =>
    api<{ projects: TrainingProject[] }>("GET", "/v1/me/training").then((r) => r.projects),
  );
  const quality = useLoad(() => api<MyQuality>("GET", "/v1/me/quality"));

  return (
    <div className="grid max-w-4xl gap-14">
      <header className="grid gap-2">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          Training
        </h1>
        <p className="max-w-2xl text-fg-muted">
          The guidelines for your projects, how your work is scored, worked examples and answers to common
          questions.
        </p>
        <nav aria-label="On this page" className="mt-2 flex flex-wrap gap-x-5 gap-y-1 text-sm">
          {[
            ["#guidelines", "Project guidelines"],
            ["#quality", "How scoring works"],
            ["#examples", "Worked examples"],
            ["#faq", "Questions"],
          ].map(([href, label]) => (
            <a key={href} href={href} className="underline underline-offset-4 hover:text-fg-muted">
              {label}
            </a>
          ))}
        </nav>
      </header>

      <section id="guidelines" aria-labelledby="guidelines-h" className="grid scroll-mt-24 gap-4">
        <h2 id="guidelines-h" className={h2}>
          Project guidelines
        </h2>
        {projects.status === "loading" && <Skeleton className="h-32 w-full" />}
        {projects.status === "error" && <Alert tone="danger" title={errorMessage(projects.error)} />}
        {projects.status === "ready" &&
          (projects.data.length === 0 ? (
            <EmptyState
              title="No project guidelines for you yet"
              action={
                <ButtonLink href="/c/qualify" variant="secondary">
                  Qualification tests
                </ButtonLink>
              }
            >
              Guidelines appear here for each project you can work on. Until then, the examples and questions
              below explain how the work is done and scored.
            </EmptyState>
          ) : (
            <Accordion
              items={projects.data.map((p, i) => ({
                id: p.id,
                open: i === 0,
                title: (
                  <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    {p.name}
                    <Tag>{taskTypeLabel[p.task_type] ?? p.task_type}</Tag>
                  </span>
                ),
                content: (
                  <div className="grid gap-4 text-fg">
                    <div className="grid gap-1">
                      <h3 className="text-sm font-semibold">Instructions</h3>
                      <p className="whitespace-pre-wrap">{p.instructions_md || "No instructions yet."}</p>
                    </div>
                    {p.guidelines.map((g) => (
                      <div key={g.title} className="grid gap-1">
                        <h3 className="text-sm font-semibold">{g.title}</h3>
                        <p className="whitespace-pre-wrap">{g.body_md}</p>
                      </div>
                    ))}
                    <div>
                      <ButtonLink href={`/c/projects/${p.id}`} variant="ghost" size="sm" arrow>
                        Project brief<span className="sr-only"> for {p.name}</span>
                      </ButtonLink>
                    </div>
                  </div>
                ),
              }))}
            />
          ))}
      </section>

      <section id="quality" aria-labelledby="quality-h" className="grid scroll-mt-24 gap-5">
        <h2 id="quality-h" className={h2}>
          How scoring works
        </h2>
        {quality.status === "ready" ? (
          <ScoreRules q={quality.data} />
        ) : quality.status === "error" ? (
          <Alert tone="danger" title={errorMessage(quality.error)} />
        ) : (
          <Skeleton className="h-40 w-full" />
        )}
      </section>

      <section id="examples" aria-labelledby="examples-h" className="grid scroll-mt-24 gap-5">
        <div className="grid gap-1">
          <h2 id="examples-h" className={h2}>
            Worked examples
          </h2>
          <p className="text-sm text-fg-muted">
            Written for this page to show good answers. They are not tasks from a real project.
          </p>
        </div>
        <div className="grid gap-5">
          {workedExamples.map((ex) => (
            <Example key={ex.id} ex={ex} />
          ))}
        </div>
      </section>

      <section id="faq" aria-labelledby="faq-h" className="grid scroll-mt-24 gap-4">
        <h2 id="faq-h" className={h2}>
          Questions
        </h2>
        <Accordion items={faqs.map((f) => ({ id: f.id, title: f.q, content: <p>{f.a}</p> }))} />
      </section>
    </div>
  );
}

function ScoreRules({ q }: { q: MyQuality }) {
  const parts = [
    {
      w: q.weights.gold_accuracy,
      name: "gold accuracy",
      what: "the share of checks with a known answer that you matched",
    },
    {
      w: q.weights.agreement,
      name: "agreement",
      what: "how often your answer matched the final, agreed answer",
    },
    {
      w: q.weights.review_acceptance,
      name: "review acceptance",
      what: "the share of your reviewed work that a reviewer accepted",
    },
  ];
  return (
    <div className="grid gap-5">
      <p className="rounded-card border border-border bg-surface p-5 font-mono text-sm leading-relaxed">
        trust = {parts.map((p, i) => `${i ? " + " : ""}${p.w.toFixed(1)} × ${p.name}`).join("")}
      </p>
      <ul className="grid gap-2 text-fg-muted">
        {parts.map((p) => (
          <li key={p.name}>
            <span className="font-medium text-fg">{p.name[0].toUpperCase() + p.name.slice(1)}</span> is{" "}
            {p.what}.
          </li>
        ))}
      </ul>
      <ul className="grid list-disc gap-2 pl-5 text-sm text-fg-muted">
        <li>Each input counts your last {q.window_days} days of work.</li>
        <li>An input with no data yet is left out, and the other two carry its weight.</li>
        <li>
          Your score is still building until you have done {q.min_gold_results} checks with known answers.
          While building, it cannot pause your account.
        </li>
        <li>
          After that, a score below {score(q.floor)} pauses new tasks until the MezAI team has looked at your
          recent work.
        </li>
        <li>Scores are shown as they stood at the start of the day, so today&apos;s work shows tomorrow.</li>
      </ul>
    </div>
  );
}

function Example({ ex }: { ex: WorkedExample }) {
  return (
    <Card data-demo className="grid gap-4 p-5 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="font-display text-lg font-semibold">{ex.title}</h3>
        <Tag>Worked example</Tag>
      </div>
      <p className="text-sm text-fg-muted">{ex.instruction}</p>
      {ex.taskType === "classification" && (
        <>
          <p className="border-l-2 border-border pl-4">{ex.text}</p>
          <ul className="flex flex-wrap gap-2" aria-label="Labels">
            {ex.labels.map((l) => (
              <li
                key={l}
                className={
                  l === ex.answer
                    ? "rounded-full border border-fg bg-fg px-3 py-1 text-sm text-bg"
                    : "rounded-full border border-border px-3 py-1 text-sm text-fg-muted"
                }
              >
                {l}
                {l === ex.answer && <span className="sr-only"> (chosen)</span>}
              </li>
            ))}
          </ul>
        </>
      )}
      {ex.taskType === "pairwise" && (
        <>
          <p className="font-medium">“{ex.prompt}”</p>
          <div className="grid gap-3 md:grid-cols-2">
            {(["A", "B"] as const).map((k) => (
              <div
                key={k}
                className={
                  k === ex.answer
                    ? "rounded-card border border-fg p-4"
                    : "rounded-card border border-border p-4"
                }
              >
                <p className="mb-1 font-mono text-xs text-fg-muted">
                  Response {k}
                  {k === ex.answer && " · chosen"}
                </p>
                <p className="text-sm">{k === "A" ? ex.a : ex.b}</p>
              </div>
            ))}
          </div>
          <p className="text-sm">
            <span className="font-semibold">Rationale: </span>
            {ex.rationale}
          </p>
        </>
      )}
      {ex.taskType === "translation" && (
        <dl className="grid gap-3 text-sm sm:grid-cols-[9rem_1fr]">
          <dt className="text-fg-muted">English</dt>
          <dd>{ex.source}</dd>
          <dt className="text-fg-muted">Machine translation</dt>
          <dd>{ex.machine}</dd>
          <dt className="text-fg-muted">Scores</dt>
          <dd className="font-mono">
            adequacy {ex.adequacy}/5 · fluency {ex.fluency}/5
          </dd>
          <dt className="text-fg-muted">Corrected</dt>
          <dd>{ex.corrected}</dd>
        </dl>
      )}
      <p className="border-t border-border pt-3 text-sm text-fg-muted">{ex.why}</p>
    </Card>
  );
}
