"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox, ChoiceGroup, Radio } from "@/components/ui/choice";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { domainOptions, labelFor, languageOptions } from "@/content/profile";
import { ApiError, api, type GoldItem, type QualificationTest, type TaskTemplate } from "@/lib/api/client";
import { errorMessage, type TaskType } from "@/lib/onboarding";
import { taskTypeLabel, taskTypes } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { pct, shortDate } from "../contributors";
import { useProject } from "./frame";
import { ImportPanel } from "./imports";

const preferenceLabel: Record<string, string> = { a: "Response A", b: "Response B", tie: "Tie" };

/** The expected answer in a few words, for the table. */
export function expectedText(type: string, e: Record<string, unknown>): string {
  if (type === "classification") return ((e.labels as string[]) ?? []).join(", ");
  if (type === "pairwise") return preferenceLabel[String(e.preference)] ?? String(e.preference);
  const base = `Adequacy ${e.adequacy}, fluency ${e.fluency}`;
  return e.corrected ? `${base}, with correction` : base;
}

const firstText = (c: Record<string, string>) => c.text ?? c.prompt ?? c.source ?? Object.values(c)[0] ?? "";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

function AddGold({ open, onClose, onAdded }: { open: boolean; onClose: () => void; onAdded: () => void }) {
  const { project } = useProject();
  const type = project.task_type as TaskType;
  const templates = useLoad(() =>
    api<{ templates: TaskTemplate[] }>("GET", "/v1/task-templates").then((r) => r.templates),
  );
  const fields =
    templates.status === "ready"
      ? (templates.data.find((t) => t.task_type === type)?.content_fields ?? [])
      : [];
  const [content, setContent] = useState<Record<string, string>>({});
  const [externalId, setExternalId] = useState("");
  const [labels, setLabels] = useState<string[]>([]);
  const [preference, setPreference] = useState("");
  const [adequacy, setAdequacy] = useState("");
  const [fluency, setFluency] = useState("");
  const [corrected, setCorrected] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const toast = useToast();

  function expected() {
    if (type === "classification") return { labels };
    if (type === "pairwise") return { preference };
    return {
      adequacy: Number(adequacy),
      fluency: Number(fluency),
      ...(corrected.trim() ? { corrected } : {}),
    };
  }

  async function save() {
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/projects/${project.id}/gold`, {
        external_id: externalId.trim() || undefined,
        content,
        expected: expected(),
      });
      toast({ tone: "success", title: "Gold item added" });
      setContent({});
      setExternalId("");
      setLabels([]);
      setPreference("");
      setAdequacy("");
      setFluency("");
      setCorrected("");
      onAdded();
      onClose();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  const fieldError = (f: string) => (error?.field === f ? errorMessage(error) : undefined);
  const multi = Boolean(project.config.multi_label);
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Add a gold item"
      description="An item with a known correct answer. Contributors cannot tell gold from ordinary items."
      className="w-[min(40rem,calc(100vw-2rem))]"
      footer={
        <>
          <Button variant="secondary" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={save} disabled={busy}>
            {busy ? "Saving…" : "Add gold item"}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        {fields.map((f) => (
          <Field
            key={f}
            label={<span className="font-mono text-xs">{f}</span>}
            required
            error={fieldError(f)}
          >
            {f.endsWith("_lang") ? (
              <Input
                value={content[f] ?? ""}
                placeholder="sw"
                onChange={(e) => setContent({ ...content, [f]: e.target.value })}
              />
            ) : (
              <Textarea
                rows={3}
                value={content[f] ?? ""}
                onChange={(e) => setContent({ ...content, [f]: e.target.value })}
              />
            )}
          </Field>
        ))}
        {type === "classification" && (
          <ChoiceGroup
            legend={multi ? "Correct labels" : "Correct label"}
            error={fieldError("expected.labels")}
          >
            {(project.config.labels ?? []).map((l) =>
              multi ? (
                <Checkbox
                  key={l}
                  label={l}
                  checked={labels.includes(l)}
                  onChange={(e) =>
                    setLabels(e.target.checked ? [...labels, l] : labels.filter((x) => x !== l))
                  }
                />
              ) : (
                <Radio
                  key={l}
                  name="gold-label"
                  label={l}
                  checked={labels[0] === l}
                  onChange={() => setLabels([l])}
                />
              ),
            )}
          </ChoiceGroup>
        )}
        {type === "pairwise" && (
          <ChoiceGroup legend="Better response" error={fieldError("expected.preference")}>
            {Object.entries(preferenceLabel).map(([v, l]) => (
              <Radio
                key={v}
                name="gold-pref"
                label={l}
                checked={preference === v}
                onChange={() => setPreference(v)}
              />
            ))}
          </ChoiceGroup>
        )}
        {type === "translation_review" && (
          <>
            <div className="grid gap-4 sm:grid-cols-2">
              {(
                [
                  ["Adequacy (1 to 5)", adequacy, setAdequacy],
                  ["Fluency (1 to 5)", fluency, setFluency],
                ] as const
              ).map(([label, v, set]) => (
                <Field key={label} label={label} required error={fieldError("expected")}>
                  <Select value={v} onChange={(e) => set(e.target.value)}>
                    <option value="">Choose</option>
                    {[1, 2, 3, 4, 5].map((n) => (
                      <option key={n}>{n}</option>
                    ))}
                  </Select>
                </Field>
              ))}
            </div>
            <Field
              label="Reference correction"
              hint="Shown to reviewers; not scored."
              error={fieldError("expected.corrected")}
            >
              <Textarea rows={3} value={corrected} onChange={(e) => setCorrected(e.target.value)} />
            </Field>
          </>
        )}
        <Field label="Your ID for this item" error={fieldError("external_id")}>
          <Input value={externalId} maxLength={200} onChange={(e) => setExternalId(e.target.value)} />
        </Field>
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} />}
        {error?.field &&
          !fields.includes(error.field) &&
          !error.field.startsWith("expected") &&
          error.field !== "external_id" && <Alert tone="danger" title={errorMessage(error)} />}
      </div>
    </Dialog>
  );
}

function MakeTest({
  open,
  onClose,
  activeGold,
  onMade,
}: {
  open: boolean;
  onClose: () => void;
  activeGold: number;
  onMade: () => void;
}) {
  const { project } = useProject();
  const lang = project.skills.find((s) => s.kind === "language")?.value;
  const [title, setTitle] = useState(`${project.name} qualification`);
  const [kind, setKind] = useState<"language" | "domain" | "task_type">(lang ? "language" : "task_type");
  const [value, setValue] = useState(lang ?? project.task_type);
  const [count, setCount] = useState(String(Math.min(10, activeGold)));
  const [pass, setPass] = useState("80");
  const [instructions, setInstructions] = useState(project.instructions_md);
  const [shareable, setShareable] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const toast = useToast();

  const options =
    kind === "language"
      ? languageOptions
      : kind === "domain"
        ? domainOptions
        : taskTypes.map((t) => ({ value: t, label: taskTypeLabel[t] }));

  async function make() {
    const n = Number(count);
    const threshold = Number(pass) / 100;
    if (!Number.isInteger(n) || n < 1 || n > Math.min(50, activeGold))
      return setError(
        new ApiError(
          422,
          "invalid_input",
          "question_count",
          `Use 1 to ${Math.min(50, activeGold)} questions.`,
        ),
      );
    if (!(threshold > 0 && threshold <= 1))
      return setError(
        new ApiError(422, "invalid_input", "pass_threshold", "Use a pass mark from 1 to 100 percent."),
      );
    if (!shareable)
      return setError(
        new ApiError(
          422,
          "invalid_input",
          "content_shareable",
          "Confirm the content may be shown to applicants.",
        ),
      );
    setBusy(true);
    setError(null);
    try {
      const t = await api<QualificationTest>("POST", "/v1/qualification/tests", {
        title: title.trim(),
        skill_kind: kind,
        skill_value: value,
        pass_threshold: threshold,
        instructions_md: instructions,
        project_id: project.id,
        question_count: n,
        content_shareable: true,
      });
      toast({ tone: "success", title: `Draft test made with ${t.question_count} questions` });
      onMade();
      onClose();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  const fieldError = (f: string) => (error?.field === f ? errorMessage(error) : undefined);
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Make a qualification test"
      description="Copies randomly chosen active gold items into a draft test. Those items are then retired here, because applicants will have seen them."
      className="w-[min(40rem,calc(100vw-2rem))]"
      footer={
        <>
          <Button variant="secondary" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={make} disabled={busy}>
            {busy ? "Making…" : "Make draft test"}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <Field label="Title" required error={fieldError("title")}>
          <Input value={title} maxLength={200} onChange={(e) => setTitle(e.target.value)} />
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Qualifies for" required>
            <Select
              value={kind}
              onChange={(e) => {
                const k = e.target.value as typeof kind;
                setKind(k);
                setValue(k === "task_type" ? project.task_type : "");
              }}
            >
              <option value="language">A language</option>
              <option value="domain">A domain</option>
              <option value="task_type">A task type</option>
            </Select>
          </Field>
          <Field label="Skill" required error={fieldError("skill_value")}>
            <Select value={value} onChange={(e) => setValue(e.target.value)}>
              <option value="">Choose</option>
              {options.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
          </Field>
          <Field
            label="Questions"
            required
            hint={`${activeGold} active gold items available.`}
            error={fieldError("question_count")}
          >
            <Input
              type="number"
              min={1}
              max={Math.min(50, activeGold)}
              value={count}
              onChange={(e) => setCount(e.target.value)}
            />
          </Field>
          <Field label="Pass mark (%)" required error={fieldError("pass_threshold")}>
            <Input type="number" min={1} max={100} value={pass} onChange={(e) => setPass(e.target.value)} />
          </Field>
        </div>
        <Field label="Instructions for applicants" error={fieldError("instructions_md")}>
          <Textarea rows={4} value={instructions} onChange={(e) => setInstructions(e.target.value)} />
        </Field>
        <Checkbox
          label="The gold content may be shown to applicants who are not yet vetted"
          hint="Check that it holds no client-confidential or personal data."
          checked={shareable}
          onChange={(e) => setShareable(e.target.checked)}
        />
        {fieldError("content_shareable") && <Alert tone="danger" title={fieldError("content_shareable")} />}
        {error &&
          (!error.field ||
            ![
              "title",
              "skill_value",
              "question_count",
              "pass_threshold",
              "instructions_md",
              "content_shareable",
            ].includes(error.field)) && <Alert tone="danger" title={errorMessage(error)} />}
      </div>
    </Dialog>
  );
}

function Tests({ tick }: { tick: number }) {
  const { project } = useProject();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const tests = useLoad(
    () =>
      api<{ tests: QualificationTest[] }>("GET", "/v1/qualification/tests").then((r) =>
        r.tests.filter((t) => t.source_project_id === project.id),
      ),
    `${project.id}/${tick}`,
  );
  async function setStatus(t: QualificationTest, status: "active" | "retired") {
    setBusy(t.id);
    setError(null);
    try {
      await api("POST", `/v1/qualification/tests/${t.id}/status`, { status });
      tests.reload();
    } catch (e) {
      setError(errorMessage(asError(e)));
    } finally {
      setBusy(null);
    }
  }
  if (tests.status === "loading") return <Skeleton className="h-20 w-full" />;
  if (tests.status === "error") return <Alert tone="danger" title={errorMessage(tests.error)} />;
  if (tests.data.length === 0) return null;
  return (
    <section aria-labelledby="tests-h" className="grid gap-3">
      <h2 id="tests-h" className="font-display font-semibold">
        Qualification tests from this project
      </h2>
      {error && <Alert tone="danger" title={error} />}
      <Table
        caption="Qualification tests made from this project's gold"
        hideCaption
        rows={tests.data}
        rowKey={(t) => t.id}
        empty=""
        columns={[
          { key: "title", header: "Test", cell: (t) => t.title },
          { key: "skill", header: "Skill", cell: (t) => labelFor(t.skill_value) },
          { key: "q", header: "Questions", numeric: true, cell: (t) => t.question_count },
          { key: "pass", header: "Pass mark", numeric: true, cell: (t) => pct(t.pass_threshold) },
          {
            key: "status",
            header: "Status",
            cell: (t) => (
              <StatusPill status={t.status === "active" ? "success" : "neutral"}>
                {t.status === "active" ? "Open" : t.status === "draft" ? "Draft" : "Closed"}
              </StatusPill>
            ),
          },
          {
            key: "act",
            header: <span className="sr-only">Action</span>,
            cell: (t) =>
              t.status === "retired" ? null : (
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={busy === t.id}
                  onClick={() => setStatus(t, t.status === "draft" ? "active" : "retired")}
                >
                  {t.status === "draft" ? "Open to applicants" : "Close"}
                  <span className="sr-only"> {t.title}</span>
                </Button>
              ),
          },
        ]}
      />
    </section>
  );
}

export function ProjectGold() {
  const { project, reload } = useProject();
  const [tick, setTick] = useState(0);
  const [adding, setAdding] = useState(false);
  const [making, setMaking] = useState(false);
  const [retiring, setRetiring] = useState<GoldItem | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();
  const gold = useLoad(
    () => api<{ gold: GoldItem[] }>("GET", `/v1/projects/${project.id}/gold`).then((r) => r.gold),
    `${project.id}/${tick}`,
  );
  const refresh = () => {
    setTick((t) => t + 1);
    reload();
  };
  const closed = project.status === "closed";
  const active = gold.status === "ready" ? gold.data.filter((g) => g.status === "active").length : 0;

  async function retire() {
    if (!retiring) return;
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/gold/${retiring.id}/retire`);
      toast({ tone: "success", title: "Gold item retired" });
      setRetiring(null);
      refresh();
    } catch (e) {
      setError(errorMessage(asError(e)));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <p className="max-w-2xl text-sm text-fg-muted">
          Gold items have a known answer. They are mixed into the work at{" "}
          {Math.round(project.gold_rate * 1000) / 10}% and score each contributor&apos;s accuracy. Retired
          items stop being served; past results stay.
        </p>
        {!closed && (
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="secondary" onClick={() => setAdding(true)}>
              Add one by hand
            </Button>
            <Button size="sm" variant="secondary" onClick={() => setMaking(true)} disabled={active === 0}>
              Make a qualification test
            </Button>
          </div>
        )}
      </div>

      <ImportPanel kind="gold" onImported={refresh} />

      <section aria-labelledby="gold-h" className="grid gap-3">
        <h2 id="gold-h" className="font-display font-semibold">
          Gold set{gold.status === "ready" && ` (${active} active)`}
        </h2>
        {error && <Alert tone="danger" title={error} />}
        {gold.status === "loading" && <Skeleton className="h-40 w-full" />}
        {gold.status === "error" && <Alert tone="danger" title={errorMessage(gold.error)} />}
        {gold.status === "ready" && (
          <Table
            caption="Gold items with their expected answers"
            hideCaption
            rows={gold.data}
            rowKey={(g) => g.id}
            empty="No gold items yet. Import a file or add one by hand."
            columns={[
              {
                key: "item",
                header: "Item",
                cell: (g) => (
                  <span className="grid max-w-md gap-0.5">
                    {g.external_id && (
                      <span className="font-mono text-xs text-fg-muted">{g.external_id}</span>
                    )}
                    <span className="line-clamp-2 break-words">{firstText(g.content)}</span>
                  </span>
                ),
              },
              { key: "exp", header: "Expected", cell: (g) => expectedText(project.task_type, g.expected) },
              {
                key: "status",
                header: "Status",
                cell: (g) => (
                  <StatusPill status={g.status === "active" ? "success" : "neutral"}>
                    {g.status === "active" ? "Active" : "Retired"}
                  </StatusPill>
                ),
                sortValue: (g) => g.status,
              },
              {
                key: "used",
                header: "Served",
                numeric: true,
                cell: (g) => g.usage_count,
                sortValue: (g) => g.usage_count,
              },
              {
                key: "at",
                header: "Added",
                cell: (g) => shortDate(g.created_at),
                sortValue: (g) => g.created_at,
              },
              {
                key: "act",
                header: <span className="sr-only">Action</span>,
                cell: (g) =>
                  g.status === "active" && !closed ? (
                    <Button size="sm" variant="ghost" className="-my-2" onClick={() => setRetiring(g)}>
                      Retire<span className="sr-only"> {g.external_id ?? "gold item"}</span>
                    </Button>
                  ) : null,
              },
            ]}
          />
        )}
      </section>

      <Tests tick={tick} />

      <AddGold open={adding} onClose={() => setAdding(false)} onAdded={refresh} />
      {making && <MakeTest open onClose={() => setMaking(false)} activeGold={active} onMade={refresh} />}
      <Dialog
        open={retiring != null}
        onClose={() => setRetiring(null)}
        title="Retire this gold item?"
        description="It stops being served to contributors. Results already recorded stay. This cannot be undone."
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setRetiring(null)}>
              Cancel
            </Button>
            <Button variant="danger" size="sm" onClick={retire} disabled={busy}>
              Retire
            </Button>
          </>
        }
      />
    </div>
  );
}
