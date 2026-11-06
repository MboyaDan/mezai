"use client";

import { useState, type ReactNode } from "react";
import { Plus, X } from "lucide-react";
import { Button, IconButton } from "@/components/ui/button";
import { Checkbox, ChoiceGroup, Radio } from "@/components/ui/choice";
import { Alert, Card } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { domainOptions, languageOptions, proficiencyOptions } from "@/content/profile";
import {
  ApiError,
  api,
  type Organisation,
  type Project,
  type ProjectInput,
  type ProjectSkill,
} from "@/lib/api/client";
import { errorMessage, type TaskType } from "@/lib/onboarding";
import { taskTypeLabel, taskTypes, toInput, type FormErrors, type ProjectForm } from "@/lib/projects";

function Section({ title, hint, children }: { title: string; hint?: ReactNode; children: ReactNode }) {
  return (
    <Card className="grid gap-5 p-5">
      <div className="grid gap-1">
        <h2 className="font-display font-semibold">{title}</h2>
        {hint && <p className="text-sm text-fg-muted">{hint}</p>}
      </div>
      {children}
    </Card>
  );
}

const skillValues = (kind: ProjectSkill["kind"]) =>
  kind === "language"
    ? languageOptions
    : kind === "domain"
      ? domainOptions
      : taskTypes.map((t) => ({ value: t, label: taskTypeLabel[t] }));

function Skills({
  value,
  onChange,
  error,
}: {
  value: ProjectSkill[];
  onChange: (s: ProjectSkill[]) => void;
  error?: string;
}) {
  const set = (i: number, s: ProjectSkill) => onChange(value.map((v, j) => (j === i ? s : v)));
  return (
    <fieldset className="grid gap-3">
      <legend className="mb-1 text-sm font-medium">
        Required skills <span className="ml-1.5 text-xs font-normal text-fg-muted">(optional)</span>
      </legend>
      <p className="text-sm text-fg-muted">
        Only contributors who hold every skill listed here receive this project&apos;s tasks.
      </p>
      {value.map((s, i) => (
        <div
          key={i}
          className="grid grid-cols-[1fr_auto] items-end gap-2 sm:grid-cols-[10rem_1fr_10rem_auto]"
        >
          <Field label="Kind" hideLabel={i > 0} className="col-span-2 sm:col-span-1">
            <Select
              value={s.kind}
              onChange={(e) => set(i, { kind: e.target.value as ProjectSkill["kind"], value: "" })}
            >
              <option value="language">Language</option>
              <option value="domain">Domain</option>
              <option value="task_type">Task type</option>
            </Select>
          </Field>
          <Field
            label="Skill"
            hideLabel={i > 0}
            required
            className={s.kind === "language" ? "" : "sm:col-span-2"}
          >
            <Select value={s.value} onChange={(e) => set(i, { ...s, value: e.target.value })}>
              <option value="">Choose</option>
              {skillValues(s.kind).map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
          </Field>
          {s.kind === "language" && (
            <Field label="At least" hideLabel={i > 0} className="col-start-1 sm:col-start-auto">
              <Select
                value={s.min_proficiency ?? ""}
                onChange={(e) =>
                  set(i, {
                    ...s,
                    min_proficiency: (e.target.value || undefined) as ProjectSkill["min_proficiency"],
                  })
                }
              >
                <option value="">Any level</option>
                {proficiencyOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          <IconButton
            label={`Remove skill ${i + 1}`}
            icon={<X />}
            onClick={() => onChange(value.filter((_, j) => j !== i))}
            className="row-start-2 sm:row-start-auto"
          />
        </div>
      ))}
      <div>
        <Button
          variant="secondary"
          size="sm"
          disabled={value.length >= 20}
          onClick={() => onChange([...value, { kind: "language", value: "" }])}
        >
          <Plus aria-hidden="true" className="size-4" /> Add a skill
        </Button>
      </div>
      {error && <p className="text-sm font-medium text-danger-fg">{error}</p>}
    </fieldset>
  );
}

export type FormMode =
  | { kind: "create"; organisations: Organisation[] }
  | { kind: "edit"; project: Project }
  | { kind: "convert"; requestId: string; orgName: string; taskType?: TaskType };

/**
 * Create, edit or convert. Organisation and task type are chosen once; labels,
 * rubric and overlap lock when the project has gold or has been live.
 */
export function ProjectFormView({
  mode,
  initial,
  onSaved,
  onCancel,
}: {
  mode: FormMode;
  initial: ProjectForm;
  onSaved: (p: Project) => void;
  onCancel?: () => void;
}) {
  const [f, setF] = useState(initial);
  const [errors, setErrors] = useState<FormErrors>({});
  const [apiError, setApiError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const creating = mode.kind !== "edit";
  const typeFixed = mode.kind === "edit" || (mode.kind === "convert" && mode.taskType != null);
  const locked = mode.kind === "edit" && Boolean(mode.project.staff?.config_locked);
  const closed = mode.kind === "edit" && mode.project.status === "closed";
  const type = f.task_type;
  const up = (patch: Partial<ProjectForm>) => setF((x) => ({ ...x, ...patch }));
  const err = (k: string) => errors[k];

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setApiError(null);
    const { input, errors: errs } = toInput(f, mode.kind === "create");
    // Skills with no value chosen are unfinished rows, not requirements.
    if (input) input.skills = (input.skills ?? []).filter((s) => s.value);
    if (mode.kind === "convert" && input) input.task_type = f.task_type;
    setErrors(errs);
    if (!input) return;
    setBusy(true);
    try {
      const path: `/v1/${string}` =
        mode.kind === "edit"
          ? `/v1/projects/${mode.project.id}`
          : mode.kind === "convert"
            ? `/v1/project-requests/${mode.requestId}/convert`
            : "/v1/projects";
      const p = await api<Project>(
        mode.kind === "edit" ? "PATCH" : "POST",
        path,
        input satisfies ProjectInput,
      );
      onSaved(p);
    } catch (x) {
      const ae = x instanceof ApiError ? x : new ApiError(0, "network");
      if (ae.field) setErrors({ [ae.field]: errorMessage(ae) });
      else setApiError(errorMessage(ae));
    } finally {
      setBusy(false);
    }
  }

  const unknownFieldError = Object.entries(errors).find(
    ([k]) =>
      (!known.has(k) || (k === "task_type" && typeFixed)) &&
      !k.startsWith("qa_checks") &&
      !k.startsWith("config"),
  );

  return (
    <form onSubmit={submit} noValidate className="grid max-w-3xl gap-6">
      {closed && <Alert tone="info" title="This project is closed. Its settings can no longer change." />}
      <fieldset disabled={closed} className="grid gap-6">
        <Section title="Basics">
          {mode.kind === "create" && (
            <Field label="Client organisation" required error={err("org_id")}>
              <Select value={f.org_id} onChange={(e) => up({ org_id: e.target.value })}>
                <option value="">Choose</option>
                {mode.organisations
                  .filter((o) => o.status === "active")
                  .map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name}
                    </option>
                  ))}
              </Select>
            </Field>
          )}
          {mode.kind === "convert" && (
            <p className="text-sm">
              <span className="text-fg-muted">Organisation:</span> {mode.orgName}
            </p>
          )}
          {typeFixed ? (
            <p className="text-sm">
              <span className="text-fg-muted">Task type:</span> {taskTypeLabel[type]}
            </p>
          ) : (
            <ChoiceGroup legend="Task type" hint="Fixed once the project exists." error={err("task_type")}>
              {taskTypes.map((t) => (
                <Radio
                  key={t}
                  name="task_type"
                  label={taskTypeLabel[t]}
                  checked={type === t}
                  onChange={() =>
                    up({
                      task_type: t,
                      min_rationale_chars: t === "pairwise" ? "40" : "0",
                      qa: {
                        ...f.qa,
                        block_unchanged_translation: t === "translation_review",
                        warn_rationale_mismatch: t === "pairwise",
                        warn_pasted: t !== "classification",
                      },
                    })
                  }
                />
              ))}
            </ChoiceGroup>
          )}
          <Field label="Project name" required error={err("name")}>
            <Input value={f.name} maxLength={200} onChange={(e) => up({ name: e.target.value })} />
          </Field>
          <Field
            label="Contributor instructions"
            hint="Plain text or Markdown. Contributors read this before every session. Needed before the project can be marked ready."
            error={err("instructions_md")}
          >
            <Textarea
              rows={10}
              value={f.instructions_md}
              maxLength={50000}
              onChange={(e) => up({ instructions_md: e.target.value })}
            />
          </Field>
        </Section>

        <Section
          title={type === "classification" ? "Labels" : type === "pairwise" ? "Rubric" : "Error tags"}
          hint={
            locked
              ? "Fixed now that the project has gold or has been live, so scores stay comparable."
              : undefined
          }
        >
          <fieldset disabled={locked} className="grid gap-4">
            {type === "classification" && (
              <>
                <Field
                  label="Labels"
                  required
                  hint="One per line, 2 to 50."
                  error={err("config.labels") ?? err("config")}
                >
                  <Textarea rows={6} value={f.labels} onChange={(e) => up({ labels: e.target.value })} />
                </Field>
                <Checkbox
                  label="Allow more than one label per item"
                  checked={f.multi_label}
                  onChange={(e) => up({ multi_label: e.target.checked })}
                />
              </>
            )}
            {type === "pairwise" && (
              <Field
                label="Rubric criteria"
                required
                hint="One per line, 1 to 8. Each response is scored 1 to 5 on each."
                error={err("config.criteria") ?? err("config")}
              >
                <Textarea rows={5} value={f.criteria} onChange={(e) => up({ criteria: e.target.value })} />
              </Field>
            )}
            {type === "translation_review" && (
              <>
                <p className="text-sm text-fg-muted">
                  Contributors score adequacy and fluency from 1 to 5 and correct the translation.
                </p>
                <Field
                  label="Error tags"
                  hint="One per line, up to 30, for example mistranslation or omission."
                  error={err("config.error_tags") ?? err("config")}
                >
                  <Textarea
                    rows={4}
                    value={f.error_tags}
                    onChange={(e) => up({ error_tags: e.target.value })}
                  />
                </Field>
              </>
            )}
          </fieldset>
        </Section>

        <Section title="Quality and pay">
          <div className="grid gap-4 sm:grid-cols-2">
            <Field
              label="Contributors per item"
              required
              hint={locked ? "Fixed now." : "Overlap for agreement. 3 is a good default."}
              error={err("overlap_k")}
            >
              <Select
                value={f.overlap_k}
                disabled={locked}
                onChange={(e) => up({ overlap_k: e.target.value })}
              >
                {[1, 2, 3, 4, 5].map((n) => (
                  <option key={n}>{n}</option>
                ))}
              </Select>
            </Field>
            <Field label="Gold mixed in (%)" required hint="0 to 50. 10% is common." error={err("gold_rate")}>
              <Input
                inputMode="decimal"
                value={f.gold_percent}
                onChange={(e) => up({ gold_percent: e.target.value })}
              />
            </Field>
            <Field
              label="Pay per approved task (KES)"
              required
              hint="For example 12.50."
              error={err("pay_per_task_cents")}
            >
              <Input
                inputMode="decimal"
                value={f.pay_kes}
                onChange={(e) => up({ pay_kes: e.target.value })}
              />
            </Field>
            <Field label="Target items" error={err("target_count")}>
              <Input
                inputMode="numeric"
                value={f.target_count}
                onChange={(e) => up({ target_count: e.target.value })}
              />
            </Field>
            <Field label="Deadline" error={err("deadline")}>
              <Input type="date" value={f.deadline} onChange={(e) => up({ deadline: e.target.value })} />
            </Field>
            {type === "pairwise" && (
              <Field label="Minimum rationale (characters)" required error={err("min_rationale_chars")}>
                <Input
                  inputMode="numeric"
                  value={f.min_rationale_chars}
                  onChange={(e) => up({ min_rationale_chars: e.target.value })}
                />
              </Field>
            )}
          </div>
        </Section>

        <Section
          title="Automatic checks before submit"
          hint="Blocking checks stop a submission. Warnings let it through and are shown to reviewers."
        >
          <div className="grid gap-4">
            {type === "translation_review" && (
              <Checkbox
                label="Block an unchanged translation scored below 5"
                checked={f.qa.block_unchanged_translation}
                onChange={(e) => up({ qa: { ...f.qa, block_unchanged_translation: e.target.checked } })}
              />
            )}
            {type === "pairwise" && (
              <Checkbox
                label="Warn when the rationale seems to favour the other response"
                checked={f.qa.warn_rationale_mismatch}
                onChange={(e) => up({ qa: { ...f.qa, warn_rationale_mismatch: e.target.checked } })}
              />
            )}
            <Checkbox
              label="Warn when most of a written answer was pasted"
              checked={f.qa.warn_pasted}
              onChange={(e) => up({ qa: { ...f.qa, warn_pasted: e.target.checked } })}
            />
            <Field
              label="Warn when a task is finished faster than (seconds)"
              required
              hint="0 turns this off."
              error={err("qa_checks.warn_fast_seconds")}
              className="max-w-xs"
            >
              <Input
                inputMode="numeric"
                value={String(f.qa.warn_fast_seconds)}
                onChange={(e) =>
                  up({ qa: { ...f.qa, warn_fast_seconds: Number(e.target.value.replace(/\D/g, "") || 0) } })
                }
              />
            </Field>
            {Object.entries(errors)
              .filter(([k]) => k.startsWith("qa_checks") && k !== "qa_checks.warn_fast_seconds")
              .map(([k, v]) => (
                <Alert key={k} tone="danger" title={v} />
              ))}
          </div>
        </Section>

        <Section title="Who can work on it">
          <Skills value={f.skills} onChange={(skills) => up({ skills })} error={err("skills")} />
        </Section>
      </fieldset>

      {unknownFieldError && <Alert tone="danger" title={unknownFieldError[1]} />}
      {Object.keys(errors).length > 0 && !unknownFieldError && (
        <Alert tone="danger" title="Some fields need attention. They are marked above." />
      )}
      {apiError && <Alert tone="danger" title={apiError} />}
      {!closed && (
        <div className="flex flex-wrap gap-2">
          <Button type="submit" disabled={busy}>
            {busy ? "Saving…" : creating ? "Create draft project" : "Save changes"}
          </Button>
          {onCancel && (
            <Button variant="secondary" onClick={onCancel}>
              Cancel
            </Button>
          )}
        </div>
      )}
    </form>
  );
}

const known = new Set([
  "org_id",
  "task_type",
  "name",
  "instructions_md",
  "overlap_k",
  "gold_rate",
  "pay_per_task_cents",
  "target_count",
  "deadline",
  "min_rationale_chars",
  "skills",
]);
