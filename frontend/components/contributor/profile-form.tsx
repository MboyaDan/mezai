"use client";

import { useState, type FormEvent } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button, ButtonLink, IconButton } from "@/components/ui/button";
import { Checkbox, ChoiceGroup } from "@/components/ui/choice";
import { Alert, Card } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import {
  countryOptions,
  domainOptions,
  educationOptions,
  languageOptions,
  proficiencyOptions,
} from "@/content/profile";
import { ApiError, api, type ContributorProfile, type ProfileInput } from "@/lib/api/client";
import { errorMessage, maskedPreview, normaliseKenyanMobile } from "@/lib/onboarding";
import { useProfile } from "./portal";

type Lang = { language: string; proficiency: string; dialects: string };
type Errors = Partial<Record<string, string>>;

function Section({ title, hint, children }: { title: string; hint?: string; children: React.ReactNode }) {
  return (
    <section className="grid gap-5 border-t border-border pt-8 md:grid-cols-12 md:gap-10">
      <div className="md:col-span-4">
        <h2 className="font-display text-lg font-semibold">{title}</h2>
        {hint && <p className="mt-1 text-sm text-fg-muted">{hint}</p>}
      </div>
      <div className="grid gap-5 md:col-span-8">{children}</div>
    </section>
  );
}

export function ProfileForm() {
  const { profile, setProfile } = useProfile();
  const toast = useToast();
  const [name, setName] = useState(profile.display_name);
  const [country, setCountry] = useState(profile.country ?? "KE");
  const [city, setCity] = useState(profile.city ?? "");
  const [education, setEducation] = useState(profile.education_level ?? "");
  const [hours, setHours] = useState(profile.weekly_hours?.toString() ?? "");
  const [langs, setLangs] = useState<Lang[]>(
    profile.languages.length
      ? profile.languages.map((l) => ({ ...l, dialects: l.dialects.join(", ") }))
      : [{ language: "", proficiency: "", dialects: "" }],
  );
  const [domains, setDomains] = useState<string[]>(profile.domains);
  const [phone, setPhone] = useState("");
  const [phone2, setPhone2] = useState("");
  const [errors, setErrors] = useState<Errors>({});
  const [formError, setFormError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  const [becameQualifying, setBecameQualifying] = useState(false);
  const removed = profile.status === "removed";

  function validate(): Errors {
    const e: Errors = {};
    if (!name.trim()) e.display_name = "Enter your name.";
    const h = Number(hours);
    if (hours !== "" && (!Number.isInteger(h) || h < 0 || h > 80))
      e.weekly_hours = "Enter a whole number from 0 to 80.";
    const used = langs.filter((l) => l.language);
    if (new Set(used.map((l) => l.language)).size !== used.length) e.languages = "List each language once.";
    if (used.some((l) => !l.proficiency)) e.languages = "Choose a level for each language.";
    if (phone || phone2) {
      if (!normaliseKenyanMobile(phone))
        e.mpesa_phone = "Enter a Kenyan mobile number, for example 0712 345 678.";
      else if (normaliseKenyanMobile(phone) !== normaliseKenyanMobile(phone2))
        e.mpesa_phone_confirm = "The two numbers do not match.";
    }
    return e;
  }

  async function submit(ev: FormEvent) {
    ev.preventDefault();
    const e = validate();
    setErrors(e);
    setFormError(null);
    if (Object.keys(e).length) {
      document.querySelector<HTMLElement>("[aria-invalid=true]")?.focus();
      return;
    }
    const body: ProfileInput = {
      display_name: name.trim(),
      country: country || null,
      city: city.trim() || null,
      education_level: education || null,
      weekly_hours: hours === "" ? null : Number(hours),
      languages: langs
        .filter((l) => l.language)
        .map((l) => ({
          language: l.language,
          proficiency: l.proficiency as "native" | "fluent" | "working",
          dialects: l.dialects
            .split(",")
            .map((d) => d.trim())
            .filter(Boolean),
        })),
      domains,
      mpesa_phone: phone,
      mpesa_phone_confirm: phone2,
    };
    setBusy(true);
    try {
      const before = profile.status;
      const next = await api<ContributorProfile>("PUT", "/v1/me/contributor/profile", body);
      setProfile(next);
      setPhone("");
      setPhone2("");
      setBecameQualifying(before === "applied" && next.status === "qualifying");
      toast({ tone: "success", title: "Profile saved" });
    } catch (err) {
      const a = err instanceof ApiError ? err : new ApiError(0, "network");
      if (a.field) setErrors({ [a.field]: errorMessage(a) });
      else setFormError(a);
    } finally {
      setBusy(false);
    }
  }

  const preview = maskedPreview(phone);

  return (
    <form onSubmit={submit} noValidate className="grid gap-8">
      <header className="grid gap-2">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          Your profile
        </h1>
        <p className="max-w-2xl text-fg-muted">
          We use this to match you to projects. Clients never see your name or number, only your contributor
          ID <span className="font-mono text-fg">{profile.public_code}</span>.
        </p>
      </header>

      {becameQualifying && (
        <Alert
          tone="success"
          title="Profile complete"
          action={
            <ButtonLink href="/c/qualify" arrow>
              Choose a qualification test
            </ButtonLink>
          }
        >
          Next, pass a short qualification test for one of your skills.
        </Alert>
      )}
      {removed && <Alert tone="danger" title="This account is closed, so the profile cannot be changed." />}
      {formError && <Alert tone="danger" title={errorMessage(formError)} />}

      <fieldset disabled={removed} className="grid gap-8">
        <Section title="About you">
          <Field label="Display name" error={errors.display_name} required>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="name"
              maxLength={120}
            />
          </Field>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label="Country" error={errors.country} required>
              <Select value={country} onChange={(e) => setCountry(e.target.value)}>
                {countryOptions.map((c) => (
                  <option key={c.value} value={c.value}>
                    {c.label}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="City or town" error={errors.city}>
              <Input
                value={city}
                onChange={(e) => setCity(e.target.value)}
                autoComplete="address-level2"
                maxLength={100}
              />
            </Field>
          </div>
          <div className="grid gap-5 sm:grid-cols-2">
            <Field label="Highest education" error={errors.education_level} required>
              <Select value={education} onChange={(e) => setEducation(e.target.value)}>
                <option value="">Choose…</option>
                {educationOptions.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </Select>
            </Field>
            <Field
              label="Hours a week you can work"
              hint="An estimate. You can change it later."
              error={errors.weekly_hours}
              required
            >
              <Input
                inputMode="numeric"
                value={hours}
                onChange={(e) => setHours(e.target.value.replace(/\D/g, ""))}
                maxLength={2}
              />
            </Field>
          </div>
        </Section>

        <Section
          title="Languages"
          hint="Every language you can read and write well enough to judge text. Native: your first language or used every day since childhood. Fluent: you read and write it easily, including nuance and slang. Working: well enough for everyday text."
        >
          {errors.languages && <p className="text-sm font-medium text-danger-fg">{errors.languages}</p>}
          <ul className="grid gap-4">
            {langs.map((l, i) => (
              <li key={i}>
                <Card className="grid gap-4 p-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
                  <Field label={`Language ${i + 1}`} required>
                    <Select
                      value={l.language}
                      onChange={(e) =>
                        setLangs((ls) => ls.map((x, j) => (j === i ? { ...x, language: e.target.value } : x)))
                      }
                    >
                      <option value="">Choose…</option>
                      {languageOptions.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </Select>
                  </Field>
                  <Field label="Level" required>
                    <Select
                      value={l.proficiency}
                      onChange={(e) =>
                        setLangs((ls) =>
                          ls.map((x, j) => (j === i ? { ...x, proficiency: e.target.value } : x)),
                        )
                      }
                    >
                      <option value="">Choose…</option>
                      {proficiencyOptions.map((o) => (
                        <option key={o.value} value={o.value}>
                          {o.label}
                        </option>
                      ))}
                    </Select>
                  </Field>
                  <IconButton
                    label={`Remove language ${i + 1}`}
                    icon={<Trash2 />}
                    variant="secondary"
                    disabled={langs.length === 1}
                    onClick={() => setLangs((ls) => ls.filter((_, j) => j !== i))}
                  />
                  <Field label="Dialects" hint="Separate with commas" className="sm:col-span-3">
                    <Input
                      value={l.dialects}
                      onChange={(e) =>
                        setLangs((ls) => ls.map((x, j) => (j === i ? { ...x, dialects: e.target.value } : x)))
                      }
                    />
                  </Field>
                </Card>
              </li>
            ))}
          </ul>
          <div>
            <Button
              variant="secondary"
              size="sm"
              icon={<Plus aria-hidden="true" className="size-4" />}
              disabled={langs.length >= 10}
              onClick={() => setLangs((ls) => [...ls, { language: "", proficiency: "", dialects: "" }])}
            >
              Add a language
            </Button>
          </div>
        </Section>

        <Section title="Domains" hint="Subjects you know well enough to judge expert answers.">
          <ChoiceGroup legend="Domains you know" error={errors.domains}>
            <div className="grid gap-3 sm:grid-cols-2">
              {domainOptions.map((d) => (
                <Checkbox
                  key={d.value}
                  label={d.label}
                  checked={domains.includes(d.value)}
                  onChange={(e) =>
                    setDomains((ds) =>
                      e.target.checked ? [...ds, d.value] : ds.filter((x) => x !== d.value),
                    )
                  }
                />
              ))}
            </div>
          </ChoiceGroup>
        </Section>

        <Section
          title="M-Pesa number"
          hint="Where approved work is paid. Enter it twice so a typo cannot send money to someone else."
        >
          {profile.mpesa_phone_masked && (
            <p className="text-sm">
              Current number: <span className="font-mono">{profile.mpesa_phone_masked}</span>. Leave both
              boxes empty to keep it.
            </p>
          )}
          <div className="grid gap-5 sm:grid-cols-2">
            <Field
              label="M-Pesa number"
              hint="For example 0712 345 678"
              error={errors.mpesa_phone}
              required={!profile.mpesa_phone_masked}
            >
              <Input
                type="tel"
                inputMode="tel"
                autoComplete="tel"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
              />
            </Field>
            <Field
              label="Enter it again"
              error={errors.mpesa_phone_confirm}
              required={!profile.mpesa_phone_masked}
            >
              <Input
                type="tel"
                inputMode="tel"
                autoComplete="off"
                value={phone2}
                onChange={(e) => setPhone2(e.target.value)}
                onPaste={(e) => e.preventDefault()}
              />
            </Field>
          </div>
          {preview && (
            <p className="text-sm text-fg-muted">
              We will show it as <span className="font-mono">{preview}</span>.
            </p>
          )}
        </Section>
      </fieldset>

      <div className="flex flex-wrap items-center gap-4 border-t border-border pt-6">
        <Button type="submit" loading={busy} disabled={removed}>
          Save profile
        </Button>
        {profile.missing.length > 0 && !removed && (
          <p className="text-sm text-fg-muted">Fill every required field to unlock qualification tests.</p>
        )}
      </div>
    </form>
  );
}
