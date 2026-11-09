"use client";

import { useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/choice";
import { Dialog, Drawer } from "@/components/ui/dialog";
import { Alert, Card, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { labelFor } from "@/content/profile";
import { ApiError, api, type Application, type ContributorProfile, type Interview } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { errorMessage } from "@/lib/onboarding";
import {
  confidentialityQuestion,
  interviewComplete,
  interviewPasses,
  scenario,
  screeningLabel,
  screeningTone,
  type InterviewForm,
} from "@/lib/screening";
import { shortDate } from "./contributors";
import { useCan } from "./workspace";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

type Decision = "shortlist" | "waitlist" | "decline";
const decisionText: Record<Decision, { label: string; hint: string }> = {
  shortlist: { label: "Shortlist", hint: "Tests open for them. Any CV is kept for 90 days." },
  waitlist: {
    label: "Waiting list",
    hint: "No project needs their skills now. The note is shown to them. Any CV is kept for 90 days.",
  },
  decline: { label: "Decline", hint: "The note is shown to them. Any CV is deleted now." },
};

/** Ops reads the application and decides (D-100 section 3.2). */
export function ApplicationPanel({
  profile: p,
  application: a,
  onChange,
}: {
  profile: ContributorProfile;
  application: Application;
  onChange: () => void;
}) {
  const can = useCan();
  const toast = useToast();
  const [decision, setDecision] = useState<Decision | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const answers: Array<[string, string | undefined]> = [
    ["Highest study", a.experience.highest_study],
    ["Relevant experience", a.experience.relevant_experience],
    ["AI data work", a.experience.ai_data_experience],
    ["Why this work", a.experience.why],
  ];
  const canDecide = can("contributor.approve") && p.status === "qualifying";

  async function decide() {
    if (decision === "decline" && !note.trim())
      return setError(new ApiError(422, "invalid_input", "note", "Tell the applicant why."));
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/contributors/${p.id}/screening`, { decision, note: note.trim() });
      toast({
        tone: "success",
        title: `${p.public_code}: ${decision ? decisionText[decision].label.toLowerCase() : ""}`,
      });
      setDecision(null);
      setNote("");
      onChange();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-display font-semibold">Application</h2>
        <StatusPill status={screeningTone[a.screening_status]}>
          {screeningLabel[a.screening_status]}
        </StatusPill>
      </div>
      <dl className="mt-3 grid gap-3 text-sm">
        {answers.map(([k, v]) => (
          <div key={k} className="grid gap-0.5">
            <dt className="text-fg-muted">{k}</dt>
            <dd className="whitespace-pre-wrap">{v?.trim() || "–"}</dd>
          </div>
        ))}
      </dl>
      <div className="mt-4 grid gap-1.5 text-sm">
        {a.linkedin_url && (
          <a
            href={a.linkedin_url}
            target="_blank"
            rel="noopener noreferrer"
            className="underline underline-offset-4"
          >
            LinkedIn profile<span className="sr-only"> (opens a new tab)</span>
          </a>
        )}
        {a.cv_file_name ? (
          <p>
            <a href={`/api/v1/contributors/${p.id}/cv`} download className="underline underline-offset-4">
              Download CV<span className="sr-only"> of {p.public_code}</span>
            </a>{" "}
            <span className="text-fg-muted">
              {a.cv_delete_after
                ? `· deleted on ${shortDate(a.cv_delete_after)}`
                : "· kept until the decision"}
            </span>
          </p>
        ) : (
          <p className="text-fg-muted">No CV.</p>
        )}
        {a.screening_note && <p className="text-fg-muted">Note: {a.screening_note}</p>}
      </div>
      {canDecide && (
        <div className="mt-5 flex flex-wrap gap-2">
          {(Object.keys(decisionText) as Decision[])
            .filter((d) => !(d === "shortlist" && a.screening_status === "shortlisted"))
            .map((d) => (
              <Button
                key={d}
                size="sm"
                // One lime action per page (D-075): the identity panel already has it.
                variant={d === "decline" ? "danger" : "secondary"}
                onClick={() => setDecision(d)}
              >
                {decisionText[d].label}
              </Button>
            ))}
        </div>
      )}
      <Dialog
        open={decision !== null}
        onClose={() => {
          setDecision(null);
          setError(null);
        }}
        title={decision ? `${decisionText[decision].label}: ${p.public_code}?` : ""}
        description={decision ? decisionText[decision].hint : ""}
        footer={
          <>
            <Button variant="secondary" onClick={() => setDecision(null)}>
              Cancel
            </Button>
            <Button variant={decision === "decline" ? "danger" : "primary"} onClick={decide} loading={busy}>
              {decision ? decisionText[decision].label : ""}
            </Button>
          </>
        }
      >
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} className="mb-4" />}
        <Field
          label="Note"
          hint={
            decision === "shortlist"
              ? "Optional. Kept in the audit log."
              : "Shown to the applicant. Be kind and specific."
          }
          error={error?.field === "note" ? errorMessage(error) : undefined}
          required={decision === "decline"}
        >
          <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} maxLength={1000} />
        </Field>
      </Dialog>
    </Card>
  );
}

/** A 1 to 4 score as a row of radio buttons. */
function Score({
  legend,
  value,
  onChange,
}: {
  legend: string;
  value: number | null;
  onChange: (n: number) => void;
}) {
  const name = useId();
  return (
    <fieldset className="grid gap-2">
      <legend className="text-sm font-medium text-fg">{legend}</legend>
      <div className="flex gap-1.5">
        {[1, 2, 3, 4].map((n) => (
          <label key={n} className="relative">
            <input
              type="radio"
              name={name}
              checked={value === n}
              onChange={() => onChange(n)}
              className="peer absolute inset-0 z-10 cursor-pointer opacity-0"
              aria-label={`${n} out of 4`}
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
    </fieldset>
  );
}

const outcomeText: Record<Interview["outcome"], string> = {
  passed: "Passed",
  failed: "Not passed",
  no_answer: "No answer",
};

/** The structured phone interview (D-100 section 3.4): same questions and rubric for everyone. */
export function InterviewPanel({
  profile: p,
  application: a,
  interviews,
  nationalIdLast3,
  onChange,
}: {
  profile: ContributorProfile;
  application: Application;
  interviews: Interview[];
  nationalIdLast3: string | null;
  onChange: () => void;
}) {
  const can = useCan();
  const toast = useToast();
  const blank = (): InterviewForm => ({
    identity_confirmed: false,
    confidentiality_ok: false,
    scenario_score: null,
    language_scores: Object.fromEntries(p.languages.map((l) => [l.language, null])),
  });
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState<InterviewForm>(blank);
  const [availability, setAvailability] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const passed = interviews.some((i) => i.outcome === "passed");
  const canRecord = can("contributor.approve") && a.screening_status === "shortlisted" && !passed;

  async function save(outcome: "completed" | "no_answer") {
    if (outcome === "completed" && !interviewComplete(form))
      return setError(
        new ApiError(422, "invalid_input", "scores", "Score the scenario and every language first."),
      );
    setBusy(true);
    setError(null);
    try {
      await api(
        "POST",
        `/v1/contributors/${p.id}/interviews`,
        outcome === "no_answer"
          ? { outcome, note: note.trim() }
          : { outcome, ...form, availability: availability.trim(), note: note.trim() },
      );
      toast({
        tone: "success",
        title:
          outcome === "no_answer"
            ? "No answer recorded"
            : interviewPasses(form)
              ? "Interview passed"
              : "Interview recorded",
      });
      setOpen(false);
      setForm(blank());
      setAvailability("");
      setNote("");
      onChange();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-display font-semibold">Phone interview</h2>
        <StatusPill status={passed ? "success" : interviews[0]?.outcome === "failed" ? "warning" : "neutral"}>
          {passed ? "Passed" : interviews[0] ? outcomeText[interviews[0].outcome] : "Not held"}
        </StatusPill>
      </div>
      {interviews.length > 0 ? (
        <ol className="mt-3 grid gap-3 text-sm">
          {interviews.map((i) => (
            <li key={i.id} className="grid gap-0.5">
              <span>
                <strong>{outcomeText[i.outcome]}</strong> · {i.interviewer} · {shortDate(i.at)}
              </span>
              {i.outcome !== "no_answer" && (
                <span className="font-mono text-xs text-fg-muted">
                  {Object.entries(i.language_scores)
                    .map(([l, v]) => `${labelFor(l)} ${v}/4`)
                    .join(" · ")}{" "}
                  · scenario {i.scenario_score}/4 · identity{" "}
                  {i.identity_confirmed ? "confirmed" : "not confirmed"} · confidentiality{" "}
                  {i.confidentiality_ok ? "ok" : "not ok"}
                </span>
              )}
              {i.note && <span className="text-fg-muted">{i.note}</span>}
            </li>
          ))}
        </ol>
      ) : (
        <p className="mt-3 text-sm text-fg-muted">
          {a.screening_status === "shortlisted"
            ? "Call the M-Pesa number on the profile. Use the script; do not record the call."
            : "Held after the application is shortlisted."}
        </p>
      )}
      {canRecord && (
        <Button size="sm" variant="secondary" className="mt-4" onClick={() => setOpen(true)}>
          Record interview
        </Button>
      )}

      <Drawer
        open={open}
        onClose={() => {
          setOpen(false);
          setError(null);
        }}
        title={`Interview: ${p.public_code}`}
        description={`Call ${p.mpesa_phone_masked || "the M-Pesa number on the profile"}. Ask every question as written.`}
        footer={
          <>
            <Button variant="ghost" onClick={() => save("no_answer")} loading={busy}>
              No answer
            </Button>
            <Button onClick={() => save("completed")} loading={busy}>
              Save interview
            </Button>
          </>
        }
      >
        <div className="grid gap-6 text-sm">
          {error && <Alert tone="danger" title={errorMessage(error)} />}
          <section className="grid gap-2">
            <h3 className="font-display font-semibold">1. Identity</h3>
            <p>“Please confirm your full name and the last three digits of your ID number.”</p>
            <p className="text-fg-muted">
              Expect: <strong className="text-fg">{p.display_name}</strong>, ID ending{" "}
              <span className="font-mono text-fg">{nationalIdLast3 ?? "–"}</span>
            </p>
            <Checkbox
              label="Name and ID digits match"
              checked={form.identity_confirmed}
              onChange={(e) => setForm({ ...form, identity_confirmed: e.target.checked })}
            />
          </section>
          <section className="grid gap-3">
            <h3 className="font-display font-semibold">2. Languages</h3>
            <p className="text-fg-muted">
              Ask two everyday questions in each language. 4: natural and accurate · 3: clear with small slips
              · 2: often unclear · 1: cannot hold the conversation.
            </p>
            {p.languages.map((l) => (
              <Score
                key={l.language}
                legend={labelFor(l.language)}
                value={form.language_scores[l.language] ?? null}
                onChange={(n) =>
                  setForm({ ...form, language_scores: { ...form.language_scores, [l.language]: n } })
                }
              />
            ))}
          </section>
          <section className="grid gap-3">
            <h3 className="font-display font-semibold">3. Scenario</h3>
            <p>“{scenario.question}”</p>
            <ul className="grid gap-1 text-fg-muted">
              {scenario.anchors.map((x) => (
                <li key={x.score}>
                  <span className="font-mono text-fg">{x.score}</span> {x.text}
                </li>
              ))}
            </ul>
            <Score
              legend="Scenario score"
              value={form.scenario_score}
              onChange={(n) => setForm({ ...form, scenario_score: n })}
            />
          </section>
          <section className="grid gap-2">
            <h3 className="font-display font-semibold">4. Confidentiality</h3>
            <p>“{confidentialityQuestion}”</p>
            <Checkbox
              label="They keep task content private"
              checked={form.confidentiality_ok}
              onChange={(e) => setForm({ ...form, confidentiality_ok: e.target.checked })}
            />
          </section>
          <section className="grid gap-3">
            <h3 className="font-display font-semibold">5. Availability and device</h3>
            <Field label="Hours, device and connection" hint="Recorded, not scored.">
              <Input value={availability} onChange={(e) => setAvailability(e.target.value)} maxLength={300} />
            </Field>
            <Field label="Note" hint="Optional. Facts only; kept with the interview.">
              <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={2} maxLength={1000} />
            </Field>
          </section>
          <p aria-live="polite" className="font-medium">
            {interviewComplete(form)
              ? interviewPasses(form)
                ? "Result if saved: passed."
                : "Result if saved: not passed (needs identity, confidentiality and 3+ everywhere)."
              : ""}
          </p>
        </div>
      </Drawer>
    </Card>
  );
}
