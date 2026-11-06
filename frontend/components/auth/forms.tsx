"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { AuthHeading } from "@/components/auth/auth-frame";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/choice";
import { Alert } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { TextLink } from "@/components/ui/text-link";
import { ApiError, api, type Me } from "@/lib/api/client";
import { consentPayload, errorMessage, legalFor, requiredConsents, safeNext } from "@/lib/onboarding";
import { useSession } from "@/lib/session";

const MIN_PASSWORD = 12;

function useSubmit() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    try {
      await fn();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }
  return { busy, error, setError, run };
}

function FormError({ error }: { error: ApiError | null }) {
  if (!error || error.field) return null;
  return <Alert tone="danger" title={errorMessage(error)} />;
}

export function LoginForm() {
  const router = useRouter();
  const params = useSearchParams();
  const { state, signedIn } = useSession();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const { busy, error, run } = useSubmit();
  const [resent, setResent] = useState(false);

  // Someone already signed in goes straight on.
  useEffect(() => {
    if (state.status === "signed-in") router.replace(safeNext(params.get("next"), state.user.kind));
  }, [state, params, router]);

  function submit(e: FormEvent) {
    e.preventDefault();
    void run(async () => {
      const me = await api<Me>("POST", "/v1/auth/login", { email, password });
      signedIn(me);
      router.replace(safeNext(params.get("next"), me.user.kind));
    });
  }

  function resend() {
    void run(async () => {
      await api("POST", "/v1/auth/verify-email/resend", { email });
      setResent(true);
    });
  }

  return (
    <>
      <AuthHeading title="Sign in">
        New contributor? <TextLink href="/join">Create an account</TextLink>.
      </AuthHeading>
      <form onSubmit={submit} noValidate className="grid gap-5">
        {error?.code === "email_not_verified" ? (
          <Alert
            tone="warning"
            title="Confirm your email address first"
            action={
              resent ? undefined : (
                <Button variant="secondary" size="sm" onClick={resend} loading={busy}>
                  Send the link again
                </Button>
              )
            }
          >
            {resent
              ? "If the address has an account waiting for confirmation, a new link is on its way."
              : "We sent you a link when you signed up."}
          </Alert>
        ) : (
          <FormError error={error} />
        )}
        <Field label="Email" required>
          <Input
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </Field>
        <Field label="Password" required>
          <Input
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </Field>
        <Button type="submit" loading={busy} fullWidth arrow>
          Sign in
        </Button>
        <p className="text-sm">
          <TextLink href="/forgot-password">Forgot your password?</TextLink>
        </p>
      </form>
    </>
  );
}

export function JoinForm() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [agreed, setAgreed] = useState<Record<string, boolean>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [done, setDone] = useState(false);
  const { busy, error, run } = useSubmit();
  const doneRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    if (done) doneRef.current?.focus();
  }, [done]);

  function submit(e: FormEvent) {
    e.preventDefault();
    const errs: Record<string, string> = {};
    if (!name.trim()) errs.name = "Enter the name you would like us to use.";
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim()))
      errs.email = "Enter an email address like name@example.com.";
    if (password.length < MIN_PASSWORD) errs.password = `Use at least ${MIN_PASSWORD} characters.`;
    for (const c of requiredConsents) {
      if (!agreed[c.key])
        errs[c.key] = `Read and accept the ${legalFor(c.key).short.toLowerCase()} to continue.`;
    }
    setErrors(errs);
    if (Object.keys(errs).length) return;
    void run(async () => {
      try {
        await api("POST", "/v1/contributors/signup", {
          email: email.trim(),
          display_name: name.trim(),
          password,
          consents: consentPayload(requiredConsents.map((c) => c.key)),
        });
        setDone(true);
      } catch (e) {
        if (e instanceof ApiError && e.code === "weak_password") {
          setErrors({ password: errorMessage(e) });
          return;
        }
        throw e;
      }
    });
  }

  if (done) {
    return (
      <div className="grid gap-4">
        <h1
          ref={doneRef}
          tabIndex={-1}
          className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] outline-none"
        >
          Check your email
        </h1>
        <p className="text-fg-muted">
          We sent a confirmation link to <strong className="text-fg">{email.trim()}</strong>. Open it to
          confirm your address, then sign in to complete your profile. The link works for 24 hours.
        </p>
        <p className="text-sm text-fg-muted">
          Nothing arrived? Check spam, or <TextLink href="/login">sign in</TextLink> to have it sent again.
        </p>
      </div>
    );
  }

  return (
    <>
      <AuthHeading title="Join as a contributor">
        Already have an account? <TextLink href="/login">Sign in</TextLink>.
      </AuthHeading>
      <form onSubmit={submit} noValidate className="grid gap-5">
        <FormError error={error} />
        <Field
          label="Display name"
          hint="Shown to the MezAI team. Clients only ever see a contributor ID."
          error={errors.name}
          required
        >
          <Input
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            maxLength={120}
            required
          />
        </Field>
        <Field label="Email" error={errors.email} required>
          <Input
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </Field>
        <Field
          label="Password"
          hint={`At least ${MIN_PASSWORD} characters. A short sentence works well.`}
          error={errors.password}
          required
        >
          <Input
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            minLength={MIN_PASSWORD}
            required
          />
        </Field>
        <fieldset className="grid gap-3 border-t border-border pt-5">
          <legend className="sr-only">Consent</legend>
          {requiredConsents.map((c) => {
            const doc = legalFor(c.key);
            return (
              <div key={c.key} className="grid gap-1">
                <Checkbox
                  name={c.key}
                  checked={!!agreed[c.key]}
                  onChange={(e) => setAgreed((a) => ({ ...a, [c.key]: e.target.checked }))}
                  aria-invalid={errors[c.key] ? true : undefined}
                  label={
                    <>
                      I have read and accept the{" "}
                      <Link
                        href={`/legal/${doc.slug}`}
                        target="_blank"
                        className="underline underline-offset-2"
                      >
                        {doc.short.toLowerCase()}
                      </Link>{" "}
                      <span className="font-mono text-xs text-fg-muted">v{doc.version}</span>
                    </>
                  }
                />
                {errors[c.key] && <p className="pl-8 text-sm font-medium text-danger-fg">{errors[c.key]}</p>}
              </div>
            );
          })}
          <p className="text-sm text-fg-muted">
            We record which version you accepted and when. The documents are drafts until reviewed by an
            advocate; we will ask you again when they change.
          </p>
        </fieldset>
        <Button type="submit" loading={busy} fullWidth arrow>
          Create account
        </Button>
      </form>
    </>
  );
}

export function VerifyEmail() {
  const params = useSearchParams();
  const token = params.get("token") ?? "";
  const [state, setState] = useState<"working" | "done" | "failed">(token ? "working" : "failed");
  const sent = useRef(false);

  useEffect(() => {
    if (!token || sent.current) return;
    sent.current = true; // React may run effects twice in development; spend the token once.
    api("POST", "/v1/auth/verify-email", { token }).then(
      () => setState("done"),
      () => setState("failed"),
    );
  }, [token]);

  if (state === "working") return <AuthHeading title="Confirming your email…" />;
  if (state === "done")
    return (
      <AuthHeading title="Email confirmed">
        <p className="mb-6">Sign in to complete your profile and take a qualification test.</p>
        <Link href="/login?next=/c" className="font-medium text-fg underline underline-offset-4">
          Sign in
        </Link>
      </AuthHeading>
    );
  return (
    <AuthHeading title="This link did not work">
      <p>
        It may have expired or already been used. <TextLink href="/login">Sign in</TextLink> and we will offer
        to send a new one.
      </p>
    </AuthHeading>
  );
}

export function ForgotPasswordForm() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const { busy, error, run } = useSubmit();

  function submit(e: FormEvent) {
    e.preventDefault();
    void run(async () => {
      await api("POST", "/v1/auth/password-reset/request", { email: email.trim() });
      setSent(true);
    });
  }

  if (sent)
    return (
      <AuthHeading title="Check your email">
        If {email.trim()} has an account, a reset link is on its way. It works for one hour.
      </AuthHeading>
    );
  return (
    <>
      <AuthHeading title="Reset your password">We will email you a link to choose a new one.</AuthHeading>
      <form onSubmit={submit} noValidate className="grid gap-5">
        <FormError error={error} />
        <Field label="Email" required>
          <Input
            type="email"
            autoComplete="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
        </Field>
        <Button type="submit" loading={busy} fullWidth>
          Send reset link
        </Button>
        <p className="text-sm">
          <TextLink href="/login">Back to sign in</TextLink>
        </p>
      </form>
    </>
  );
}

export function ResetPasswordForm() {
  const token = useSearchParams().get("token") ?? "";
  const [password, setPassword] = useState("");
  const [fieldError, setFieldError] = useState<string>();
  const [done, setDone] = useState(false);
  const { busy, error, run } = useSubmit();

  function submit(e: FormEvent) {
    e.preventDefault();
    if (password.length < MIN_PASSWORD) {
      setFieldError(`Use at least ${MIN_PASSWORD} characters.`);
      return;
    }
    setFieldError(undefined);
    void run(async () => {
      try {
        await api("POST", "/v1/auth/password-reset/confirm", { token, password });
        setDone(true);
      } catch (e) {
        if (e instanceof ApiError && e.code === "weak_password") return setFieldError(errorMessage(e));
        throw e;
      }
    });
  }

  if (done)
    return (
      <AuthHeading title="Password changed">
        You have been signed out everywhere. <TextLink href="/login">Sign in</TextLink> with your new
        password.
      </AuthHeading>
    );
  return (
    <>
      <AuthHeading title="Choose a new password" />
      <form onSubmit={submit} noValidate className="grid gap-5">
        <FormError error={error} />
        <Field label="New password" hint={`At least ${MIN_PASSWORD} characters.`} error={fieldError} required>
          <Input
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
        </Field>
        <Button type="submit" loading={busy} fullWidth>
          Save password
        </Button>
      </form>
    </>
  );
}
