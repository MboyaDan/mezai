"use client";

import { useEffect, useRef, useState, useSyncExternalStore, type FormEvent } from "react";
import Script from "next/script";
import { CircleCheck } from "lucide-react";
import { Field } from "@/components/ui/field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Alert } from "@/components/ui/display";
import { TextLink } from "@/components/ui/text-link";
import { contactPage, limits, projectTypes, timelines, volumes } from "@/content/contact";
import {
  emptyEnquiry,
  submitEnquiry,
  validateEnquiry,
  type EnquiryField,
  type EnquiryValues,
  type FieldErrors,
} from "@/lib/enquiry";

// Cloudflare Turnstile appears only when DG-07 is cleared and a site key is set.
const turnstileKey = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY ?? "";

type TurnstileApi = {
  render: (
    el: HTMLElement,
    o: { sitekey: string; callback: (t: string) => void; "expired-callback": () => void },
  ) => string;
  reset: (id?: string) => void;
};

const noop = () => () => {};
const hydrated = () => true;
const notHydrated = () => false;

const order: EnquiryField[] = [
  "name",
  "organisation",
  "email",
  "project_type",
  "languages",
  "volume",
  "timeline",
  "message",
];

const problems = {
  limited: "Too many enquiries from this connection. Wait an hour and try again.",
  challenge: "The security check did not complete. Try it again, then send.",
  failed: "The enquiry could not be sent. Check your connection and try again.",
} as const;

export function EnquiryForm() {
  const { form } = contactPage;
  const [values, setValues] = useState<EnquiryValues>(emptyEnquiry);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [problem, setProblem] = useState<keyof typeof problems | null>(null);
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  // Until React takes over, a native submit would put the details in the URL.
  const ready = useSyncExternalStore(noop, hydrated, notHydrated);
  const formRef = useRef<HTMLFormElement>(null);
  const doneRef = useRef<HTMLDivElement>(null);
  const turnstileRef = useRef<HTMLDivElement>(null);
  const widgetId = useRef<string | undefined>(undefined);

  const set = (f: keyof EnquiryValues) => (e: { target: { value: string } }) => {
    setValues((v) => ({ ...v, [f]: e.target.value }));
    if (f in errors) setErrors((er) => ({ ...er, [f]: undefined }));
  };

  const renderTurnstile = () => {
    const api = (window as unknown as { turnstile?: TurnstileApi }).turnstile;
    if (!api || !turnstileRef.current || widgetId.current) return;
    widgetId.current = api.render(turnstileRef.current, {
      sitekey: turnstileKey,
      callback: (t) => setValues((v) => ({ ...v, turnstile_token: t })),
      "expired-callback": () => setValues((v) => ({ ...v, turnstile_token: undefined })),
    });
  };

  useEffect(() => {
    if (sent) doneRef.current?.focus();
  }, [sent]);

  const focusFirst = (er: FieldErrors) => {
    const first = order.find((f) => er[f]);
    if (first) formRef.current?.querySelector<HTMLElement>(`[name="${first}"]`)?.focus();
  };

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setProblem(null);
    const found = validateEnquiry(values);
    if (Object.keys(found).length) {
      setErrors(found);
      focusFirst(found);
      return;
    }
    setSending(true);
    const res = await submitEnquiry(values);
    setSending(false);
    if (res.ok) {
      setSent(true);
      return;
    }
    if (res.kind === "fields") {
      setErrors(res.errors);
      focusFirst(res.errors);
      return;
    }
    setProblem(res.kind);
    if (res.kind === "challenge") {
      (window as unknown as { turnstile?: TurnstileApi }).turnstile?.reset(widgetId.current);
    }
  }

  if (sent) {
    return (
      <div
        ref={doneRef}
        tabIndex={-1}
        role="status"
        className="grid gap-4 border-t border-border-strong pt-8 outline-none"
      >
        <CircleCheck aria-hidden="true" className="size-8 text-success" />
        <h2 className="font-display text-[1.75rem] leading-tight font-medium">{form.sent.title}</h2>
        <p className="max-w-[34rem] text-lg text-fg-muted">{form.sent.text}</p>
        <div>
          <Button
            variant="secondary"
            onClick={() => {
              setValues(emptyEnquiry);
              widgetId.current = undefined;
              setSent(false);
            }}
          >
            {form.sent.again}
          </Button>
        </div>
      </div>
    );
  }

  const errorCount = Object.values(errors).filter(Boolean).length;

  return (
    <form
      ref={formRef}
      method="post"
      noValidate
      onSubmit={onSubmit}
      aria-labelledby="enquiry-title"
      className="grid gap-6"
    >
      <h2 id="enquiry-title" className="font-display text-[1.75rem] leading-tight font-medium">
        {form.title}
      </h2>

      {errorCount > 0 && (
        <Alert
          tone="danger"
          title={errorCount === 1 ? "One field needs attention." : `${errorCount} fields need attention.`}
        />
      )}
      {problem && <Alert tone="danger" title={problems[problem]} />}

      <div className="grid gap-6 md:grid-cols-2">
        <Field label="Your name" required error={errors.name}>
          <Input
            name="name"
            autoComplete="name"
            maxLength={limits.name}
            value={values.name}
            onChange={set("name")}
          />
        </Field>
        <Field label="Organisation" required error={errors.organisation}>
          <Input
            name="organisation"
            autoComplete="organization"
            maxLength={limits.organisation}
            value={values.organisation}
            onChange={set("organisation")}
          />
        </Field>
      </div>

      <Field label="Work email" required error={errors.email}>
        <Input
          name="email"
          type="email"
          inputMode="email"
          autoComplete="email"
          spellCheck={false}
          value={values.email}
          onChange={set("email")}
        />
      </Field>

      <Field label="Project type" required error={errors.project_type}>
        <Select name="project_type" value={values.project_type} onChange={set("project_type")}>
          <option value="" disabled>
            Choose one
          </option>
          {projectTypes.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </Select>
      </Field>

      <Field label="Languages" hint="For example: Kiswahili and English" error={errors.languages}>
        <Input
          name="languages"
          maxLength={limits.languages}
          value={values.languages}
          onChange={set("languages")}
        />
      </Field>

      <div className="grid gap-6 md:grid-cols-2">
        <Field label="Volume" error={errors.volume}>
          <Select name="volume" value={values.volume} onChange={set("volume")}>
            <option value="">Not sure / prefer not to say</option>
            {volumes.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Timeline" error={errors.timeline}>
          <Select name="timeline" value={values.timeline} onChange={set("timeline")}>
            <option value="">Not sure / prefer not to say</option>
            {timelines.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </Select>
        </Field>
      </div>

      <Field
        label="Message"
        hint="What the data is for, what a good answer looks like, anything sensitive about it."
        error={errors.message}
      >
        <Textarea
          name="message"
          rows={6}
          maxLength={limits.message}
          value={values.message}
          onChange={set("message")}
        />
      </Field>

      {/* Honeypot: off-screen and out of the tab order; people never see it. */}
      <div aria-hidden="true" className="absolute -left-[9999px] h-px w-px overflow-hidden">
        <label htmlFor="enquiry-website">Leave this field empty</label>
        <input
          id="enquiry-website"
          name="website"
          type="text"
          tabIndex={-1}
          autoComplete="off"
          value={values.website}
          onChange={set("website")}
        />
      </div>

      {turnstileKey && (
        <>
          <Script
            src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"
            strategy="afterInteractive"
            onReady={renderTurnstile}
          />
          <div ref={turnstileRef} />
        </>
      )}

      <p className="text-sm text-fg-muted">
        {form.privacy} <TextLink href={form.privacyLink.href}>{form.privacyLink.label}</TextLink>
      </p>

      <div>
        <noscript>
          <p className="mb-4 text-sm text-fg-muted">This form needs JavaScript to send.</p>
        </noscript>
        <Button type="submit" size="lg" arrow loading={sending} disabled={sending || !ready}>
          {form.submit}
        </Button>
      </div>
    </form>
  );
}
