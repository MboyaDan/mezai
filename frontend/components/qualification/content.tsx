import type { ReactNode } from "react";
import { cn } from "@/lib/cn";
import { criterionLabel, type Answer, type QuestionType } from "@/lib/onboarding";

type Content = Record<string, unknown>;
const str = (v: unknown) => (typeof v === "string" ? v : "");

function Panel({
  label,
  lang,
  children,
  className,
}: {
  label: string;
  lang?: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn("grid content-start gap-2 rounded-card border border-border bg-surface p-4", className)}
    >
      <p className="font-mono text-xs tracking-wide text-fg-muted uppercase">{label}</p>
      <div lang={lang} className="text-[1.0625rem] leading-relaxed whitespace-pre-wrap text-fg">
        {children}
      </div>
    </div>
  );
}

/** The item a question asks about, in the task template's import shape. */
export function QuestionContent({ type, content }: { type: QuestionType; content: Content }) {
  switch (type) {
    case "classification":
      return <Panel label="Text">{str(content.text)}</Panel>;
    case "pairwise":
      return (
        <div className="grid gap-4">
          <Panel label="Prompt">{str(content.prompt)}</Panel>
          <div className="grid gap-4 md:grid-cols-2">
            <Panel label="Response A">{str(content.response_a)}</Panel>
            <Panel label="Response B">{str(content.response_b)}</Panel>
          </div>
        </div>
      );
    case "writing":
      return <Panel label="Writing prompt">{str(content.prompt)}</Panel>;
    case "translation_review":
      return (
        <div className="grid gap-4 md:grid-cols-2">
          <Panel label={`Source (${str(content.source_lang)})`} lang={str(content.source_lang) || undefined}>
            {str(content.source)}
          </Panel>
          <Panel
            label={`Translation (${str(content.target_lang)})`}
            lang={str(content.target_lang) || undefined}
          >
            {str(content.translation)}
          </Panel>
        </div>
      );
  }
}

/** A submitted or expected answer, read-only (Ops review). */
export function AnswerSummary({ type, answer }: { type: QuestionType; answer: Answer | null | undefined }) {
  if (!answer) return <span className="text-fg-muted">No answer</span>;
  switch (type) {
    case "classification":
      return <span>{(answer.labels ?? []).join(", ") || "None"}</span>;
    case "pairwise":
      return (
        <span className="grid gap-1">
          <span>
            Prefers{" "}
            <strong>
              {answer.preference === "tie" ? "neither (tie)" : `response ${answer.preference?.toUpperCase()}`}
            </strong>
          </span>
          {answer.scores_a && (
            <span className="font-mono text-xs text-fg-muted">
              {Object.keys(answer.scores_a)
                .map((c) => `${criterionLabel(c)} A${answer.scores_a?.[c]}/B${answer.scores_b?.[c]}`)
                .join(" · ")}
            </span>
          )}
          {answer.rationale && <span className="text-sm text-fg-muted">“{answer.rationale}”</span>}
        </span>
      );
    case "writing":
      return <span className="whitespace-pre-wrap">{answer.text || "No answer"}</span>;
    case "translation_review":
      return (
        <span className="grid gap-1">
          <span className="font-mono">
            Adequacy {answer.adequacy ?? "–"} · Fluency {answer.fluency ?? "–"}
          </span>
          {answer.corrected && <span className="text-sm text-fg-muted">{answer.corrected}</span>}
        </span>
      );
  }
}
