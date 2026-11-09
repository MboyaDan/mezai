"use client";

import { useEffect, useId } from "react";
import { KeyboardHint } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import type { TaskConfig } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { criterionLabel, type Answer, type TaskType } from "@/lib/onboarding";

type Props = {
  type: TaskType;
  config: TaskConfig;
  content: Record<string, unknown>;
  value: Answer | undefined;
  onChange: (a: Answer) => void;
  /** Characters pasted into a free-text field (QA warning signal, D-077). */
  onPaste?: (chars: number) => void;
  /** Hint under the rationale field, for example a minimum length. */
  rationaleHint?: string;
  /** Blocking QA messages per field, shown under that field. */
  fieldErrors?: Partial<Record<string, string>>;
};

/** A row of 1 to 5 radio buttons, large enough to tap. */
function Scale({
  legend,
  value,
  onChange,
  low,
  high,
}: {
  legend: string;
  value?: number;
  onChange: (n: number) => void;
  low: string;
  high: string;
}) {
  const name = useId();
  return (
    <fieldset className="grid gap-2">
      <legend className="text-sm font-medium text-fg">{legend}</legend>
      <div className="flex items-center gap-2">
        <span className="hidden w-16 text-right text-xs text-fg-muted sm:block">{low}</span>
        <div className="flex gap-1.5">
          {[1, 2, 3, 4, 5].map((n) => (
            <label key={n} className="relative">
              <input
                type="radio"
                name={name}
                value={n}
                checked={value === n}
                onChange={() => onChange(n)}
                className="peer absolute inset-0 z-10 cursor-pointer opacity-0"
                aria-label={`${n}${n === 1 ? ` (${low})` : n === 5 ? ` (${high})` : ""}`}
              />
              <span
                aria-hidden="true"
                className={cn(
                  "grid size-11 place-items-center rounded-control border border-border bg-surface font-mono text-sm",
                  "peer-checked:border-fg peer-checked:bg-fg peer-checked:text-bg",
                  "peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-fg",
                )}
              >
                {n}
              </span>
            </label>
          ))}
        </div>
        <span className="hidden text-xs text-fg-muted sm:block">{high}</span>
      </div>
    </fieldset>
  );
}

function Chips({
  options,
  selected,
  multi,
  onToggle,
}: {
  options: string[];
  selected: string[];
  multi: boolean;
  onToggle: (l: string) => void;
}) {
  const name = useId();
  if (options.length === 0)
    return <p className="text-sm text-danger-fg">This test has no labels set up. Contact the MezAI team.</p>;
  return (
    <fieldset className="grid gap-3">
      <legend className="text-sm font-medium text-fg">
        {multi ? "Choose every label that applies" : "Choose one label"}
      </legend>
      <div className="flex flex-wrap gap-2">
        {options.map((l, i) => (
          <label key={l} className="relative">
            <input
              type={multi ? "checkbox" : "radio"}
              name={name}
              checked={selected.includes(l)}
              onChange={() => onToggle(l)}
              className="peer absolute inset-0 z-10 cursor-pointer opacity-0"
            />
            <span
              className={cn(
                "inline-flex min-h-11 items-center gap-2 rounded-full border border-border bg-surface px-4 text-[0.9375rem]",
                "peer-checked:border-fg peer-checked:bg-fg peer-checked:text-bg",
                "peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-fg",
              )}
            >
              {i < 9 && (
                <kbd aria-hidden="true" className="font-mono text-xs opacity-70">
                  {i + 1}
                </kbd>
              )}
              {l}
            </span>
          </label>
        ))}
      </div>
      <KeyboardHint keys={["1", "…", String(Math.min(options.length, 9))]} label="choose with number keys" />
    </fieldset>
  );
}

export function AnswerInput({
  type,
  config,
  content,
  value,
  onChange,
  onPaste,
  rationaleHint,
  fieldErrors,
}: Props) {
  const paste = (e: React.ClipboardEvent) => onPaste?.(e.clipboardData.getData("text").length);
  const a = value ?? {};

  // Classification: number keys pick labels, unless typing in a field.
  useEffect(() => {
    if (type !== "classification") return;
    const labels = config.labels ?? [];
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      if (
        e.metaKey ||
        e.ctrlKey ||
        e.altKey ||
        t.closest("input[type=text],textarea,select,[contenteditable]")
      )
        return;
      const n = Number(e.key);
      if (!Number.isInteger(n) || n < 1 || n > Math.min(9, labels.length)) return;
      e.preventDefault();
      const l = labels[n - 1];
      const cur = value?.labels ?? [];
      onChange({
        ...value,
        labels: config.multi_label ? (cur.includes(l) ? cur.filter((x) => x !== l) : [...cur, l]) : [l],
      });
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [type, config, value, onChange]);

  switch (type) {
    case "classification": {
      const labels = config.labels ?? [];
      const cur = a.labels ?? [];
      return (
        <Chips
          options={labels}
          selected={cur}
          multi={!!config.multi_label}
          onToggle={(l) =>
            onChange({
              ...a,
              labels: config.multi_label ? (cur.includes(l) ? cur.filter((x) => x !== l) : [...cur, l]) : [l],
            })
          }
        />
      );
    }
    case "pairwise": {
      const criteria = config.criteria ?? [];
      const set = (side: "scores_a" | "scores_b", c: string, n: number) =>
        onChange({ ...a, [side]: { ...(a[side] ?? {}), [c]: n } });
      return (
        <div className="grid gap-8">
          <div className="grid gap-8 md:grid-cols-2">
            {(["scores_a", "scores_b"] as const).map((side) => (
              <div key={side} className="grid content-start gap-5">
                <h3 className="font-display font-semibold">Response {side === "scores_a" ? "A" : "B"}</h3>
                {criteria.map((c) => (
                  <Scale
                    key={c}
                    legend={`${criterionLabel(c)}, response ${side === "scores_a" ? "A" : "B"}`}
                    value={a[side]?.[c]}
                    onChange={(n) => set(side, c, n)}
                    low="Poor"
                    high="Excellent"
                  />
                ))}
              </div>
            ))}
          </div>
          <PreferenceChoice value={a.preference} onChange={(p) => onChange({ ...a, preference: p })} />
          <Field
            label="Why?"
            hint={rationaleHint ?? "One or two sentences on what decided it."}
            error={fieldErrors?.rationale}
            required
          >
            <Textarea
              value={a.rationale ?? ""}
              onPaste={paste}
              onChange={(e) => onChange({ ...a, rationale: e.target.value })}
              rows={3}
              maxLength={4000}
            />
          </Field>
        </div>
      );
    }
    case "translation_review":
      return (
        <div className="grid gap-6">
          <div className="grid gap-6 sm:grid-cols-2">
            <Scale
              legend="Adequacy: is the meaning kept?"
              value={a.adequacy}
              onChange={(n) => onChange({ ...a, adequacy: n })}
              low="None"
              high="All"
            />
            <Scale
              legend="Fluency: does it read naturally?"
              value={a.fluency}
              onChange={(n) => onChange({ ...a, fluency: n })}
              low="Broken"
              high="Natural"
            />
          </div>
          <Field
            label="Corrected translation"
            hint="Edit the translation if it needs fixing. Leave it as it is if it is right."
            error={fieldErrors?.corrected}
          >
            <Textarea
              onPaste={paste}
              value={a.corrected ?? (typeof content.translation === "string" ? content.translation : "")}
              onChange={(e) => onChange({ ...a, corrected: e.target.value })}
              rows={3}
              maxLength={4000}
            />
          </Field>
        </div>
      );
  }
}

function PreferenceChoice({
  value,
  onChange,
}: {
  value?: "a" | "b" | "tie";
  onChange: (p: "a" | "b" | "tie") => void;
}) {
  const name = useId();
  const opts = [
    { v: "a", l: "Response A" },
    { v: "tie", l: "About the same" },
    { v: "b", l: "Response B" },
  ] as const;
  return (
    <fieldset className="grid gap-3">
      <legend className="text-sm font-medium text-fg">Which response is better overall?</legend>
      <div className="grid grid-cols-3 gap-2">
        {opts.map((o) => (
          <label key={o.v} className="relative">
            <input
              type="radio"
              name={name}
              checked={value === o.v}
              onChange={() => onChange(o.v)}
              className="peer absolute inset-0 z-10 cursor-pointer opacity-0"
            />
            <span
              className={cn(
                "grid min-h-12 place-items-center rounded-control border border-border bg-surface px-2 text-center text-sm",
                "peer-checked:border-fg peer-checked:bg-fg peer-checked:text-bg",
                "peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-fg",
              )}
            >
              {o.l}
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  );
}
