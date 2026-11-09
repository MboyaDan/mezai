"use client";

import { useState, type FormEvent } from "react";
import { Plus, Trash2 } from "lucide-react";
import { Button, ButtonLink, IconButton } from "@/components/ui/button";
import { Checkbox, ChoiceGroup } from "@/components/ui/choice";
import { Alert, Card } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import {
  countryOptions,
  domainOptions,
  educationOptions,
  languageOptions,
  proficiencyOptions,
} from "@/content/profile";
import {
  ApiError,
  api,
  upload,
  type ContributorProfile,
  type Experience,
  type ProfileInput,
} from "@/lib/api/client";
import { errorMessage, maskedPreview, normaliseKenyanMobile, normaliseNationalId } from "@/lib/onboarding";
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
  const [exp, setExp] = useState<Required<Experience>>({
    highest_study: profile.experience.highest_study ?? "",
    relevant_experience: profile.experience.relevant_experience ?? "",
    ai_data_experience: profile.experience.ai_data_experience ?? "",
    why: profile.experience.why ?? "",
  });
  const [linkedIn, setLinkedIn] = useState(profile.linkedin_url ?? "");
  const [phone, setPhone] = useState("");
  const [phone2, setPhone2] = useState("");
  const [nationalId, setNationalId] = useState("");
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
    if (nationalId && !normaliseNationalId(nationalId))
      e.national_id = "Enter your National ID number: 6 to 9 digits.";
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
      national_id: nationalId,
      experience: exp,
      linkedin_url: linkedIn.trim() || null,
    };
    setBusy(true);
    try {
      const before = profile.status;
      const next = await api<ContributorProfile>("PUT", "/v1/me/contributor/profile", body);
      setProfile(next);
      setPhone("");
      setPhone2("");
      setNationalId("");
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
          title="Application complete"
          action={
            <ButtonLink href="/c" arrow>
              Back to dashboard
            </ButtonLink>
          }
        >
          A person on the MezAI team reads it next. Qualification tests open once you are shortlisted.
        </Alert>
      )}
      {removed && <Alert tone="danger" title="This account is closed, so the profile cannot be changed." />}
      {formError && <Alert tone="danger" title={errorMessage(formError)} />}

      <fieldset disabled={removed} className="grid gap-8">
        <Section title="About you">
          <Field
            label="Display name"
            hint="Your full name as registered with M-Pesa. We check it before your first payment."
            error={errors.display_name}
            required
          >
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
          title="Your experience"
          hint="A person on the MezAI team reads these answers. Short and specific is best."
        >
          <Field
            label="Highest study or training that matters for this work"
            error={errors["experience.highest_study"]}
            required
          >
            <Input
              value={exp.highest_study}
              onChange={(e) => setExp({ ...exp, highest_study: e.target.value })}
              maxLength={200}
            />
          </Field>
          <Field
            label="Relevant experience"
            hint="For example writing, translation, editing, teaching, customer support, or work in health, law or finance."
            error={errors["experience.relevant_experience"]}
            required
          >
            <Textarea
              value={exp.relevant_experience}
              onChange={(e) => setExp({ ...exp, relevant_experience: e.target.value })}
              rows={4}
              maxLength={1000}
            />
          </Field>
          <Field label="Any earlier work on AI data" error={errors["experience.ai_data_experience"]}>
            <Textarea
              value={exp.ai_data_experience}
              onChange={(e) => setExp({ ...exp, ai_data_experience: e.target.value })}
              rows={2}
              maxLength={500}
            />
          </Field>
          <Field
            label="Why do you want to do this work?"
            hint={`About 100 words. ${800 - exp.why.length} characters left.`}
            error={errors["experience.why"]}
            required
          >
            <Textarea
              value={exp.why}
              onChange={(e) => setExp({ ...exp, why: e.target.value })}
              rows={4}
              maxLength={800}
            />
          </Field>
          <Field
            label="LinkedIn profile"
            hint="Optional. For example https://www.linkedin.com/in/your-name"
            error={errors.linkedin_url}
          >
            <Input
              type="url"
              value={linkedIn}
              onChange={(e) => setLinkedIn(e.target.value)}
              autoComplete="url"
            />
          </Field>
          <CvUpload />
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

        <Section
          title="National ID number"
          hint="So each person has one account. We do not keep the number: we store a one-way code made from it and its last 3 digits."
        >
          {profile.national_id_last3 && (
            <p className="text-sm">
              On file: ID ending <span className="font-mono">{profile.national_id_last3}</span>. Leave the box
              empty to keep it.
            </p>
          )}
          <Field
            label="National ID number"
            hint="Digits only, as printed on your ID card. No photo or document is needed."
            error={errors.national_id}
            required={!profile.national_id_last3}
          >
            <Input
              inputMode="numeric"
              autoComplete="off"
              value={nationalId}
              onChange={(e) => setNationalId(e.target.value)}
              maxLength={14}
            />
          </Field>
          <IdentityNotice status={profile.identity_status} note={profile.identity_note} />
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

/** Where the identity check stands, in the contributor's words. */
export function IdentityNotice({
  status,
  note,
}: {
  status: ContributorProfile["identity_status"];
  note: string | null;
}) {
  if (status === "verified") return <p className="text-sm text-fg-muted">Your details have been checked.</p>;
  if (status === "rejected")
    return (
      <Alert tone="danger" title="We could not confirm your details">
        {note} Correct the number above, or contact us through the contact form.
      </Alert>
    );
  return null;
}

/** Optional CV: uploads on choosing a file, separate from Save (D-100). */
function CvUpload() {
  const { profile, setProfile } = useProfile();
  const toast = useToast();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  async function send(file: File) {
    setError(null);
    if (!file.name.toLowerCase().endsWith(".pdf")) return setError("Upload your CV as a PDF.");
    if (file.size > 2 * 1024 * 1024) return setError("The file is larger than 2 MB.");
    const form = new FormData();
    form.append("cv", file);
    setBusy(true);
    try {
      setProfile(await upload<ContributorProfile>("/v1/me/contributor/cv", form, "PUT"));
      toast({ tone: "success", title: "CV uploaded" });
    } catch (e) {
      setError(errorMessage(e instanceof ApiError ? e : new ApiError(0, "network")));
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    setBusy(true);
    try {
      setProfile(await api<ContributorProfile>("DELETE", "/v1/me/contributor/cv"));
      toast({ tone: "success", title: "CV deleted" });
    } catch (e) {
      setError(errorMessage(e instanceof ApiError ? e : new ApiError(0, "network")));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="grid gap-2">
      {profile.cv_file_name ? (
        <div className="flex flex-wrap items-center gap-3 text-sm">
          <span>
            CV on file: <span className="font-mono">{profile.cv_file_name}</span>
          </span>
          <Button variant="ghost" size="sm" onClick={remove} loading={busy}>
            Delete CV
          </Button>
        </div>
      ) : null}
      <Field
        label={profile.cv_file_name ? "Replace CV" : "CV"}
        hint="Optional. PDF, up to 2 MB. Leave out your address, date of birth and photo: we do not need them. We delete it 90 days after we decide on your application, or at once if we decline it."
        error={error ?? undefined}
      >
        <Input
          type="file"
          accept="application/pdf,.pdf"
          disabled={busy}
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) void send(f);
            e.target.value = "";
          }}
        />
      </Field>
    </div>
  );
}
