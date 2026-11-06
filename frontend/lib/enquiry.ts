import { limits, projectTypes, timelines, volumes } from "@/content/contact";

export type EnquiryValues = {
  name: string;
  organisation: string;
  email: string;
  project_type: string;
  languages: string;
  volume: string;
  timeline: string;
  message: string;
  /** Honeypot: hidden from people. */
  website: string;
  turnstile_token?: string;
};

export type EnquiryField = Exclude<keyof EnquiryValues, "website" | "turnstile_token">;
export type FieldErrors = Partial<Record<EnquiryField, string>>;

export const emptyEnquiry: EnquiryValues = {
  name: "",
  organisation: "",
  email: "",
  project_type: "",
  languages: "",
  volume: "",
  timeline: "",
  message: "",
  website: "",
};

const label: Record<EnquiryField, string> = {
  name: "your name",
  organisation: "your organisation",
  email: "your email address",
  project_type: "a project type",
  languages: "Languages",
  volume: "Volume",
  timeline: "Timeline",
  message: "Message",
};

/** Turns an error code (ours or the API's) into a sentence for the field. */
export function errorText(field: EnquiryField, code: string): string {
  switch (code) {
    case "required":
      return field === "project_type" ? "Choose a project type." : `Enter ${label[field]}.`;
    case "too_long":
      return `Keep this under ${limits[field as keyof typeof limits] ?? 200} characters.`;
    case "invalid":
      return field === "email"
        ? "Enter an email address like name@example.com."
        : "Remove line breaks and special characters.";
    default:
      return "Check this field.";
  }
}

const oneOf = (v: string, opts: readonly { value: string }[]) => opts.some((o) => o.value === v);

/** The same rules as the API, so most mistakes are caught before sending. */
export function validateEnquiry(v: EnquiryValues): FieldErrors {
  const codes: Partial<Record<EnquiryField, string>> = {};
  const line = (f: "name" | "organisation" | "languages", required: boolean) => {
    const s = v[f].trim();
    if (!s && required) codes[f] = "required";
    else if ([...s].length > limits[f]) codes[f] = "too_long";
    else if (/[\u0000-\u001f\u007f-\u009f]/.test(s)) codes[f] = "invalid";
  };
  line("name", true);
  line("organisation", true);
  line("languages", false);

  const email = v.email.trim();
  if (!email) codes.email = "required";
  else if (email.length > 254 || !/^[^\s@<>()",;]+@[^\s@<>()",;]+\.[^\s@<>()",;]+$/.test(email))
    codes.email = "invalid";

  if (!oneOf(v.project_type, projectTypes)) codes.project_type = "required";
  if (v.volume && !oneOf(v.volume, volumes)) codes.volume = "invalid";
  if (v.timeline && !oneOf(v.timeline, timelines)) codes.timeline = "invalid";
  if ([...v.message.trim()].length > limits.message) codes.message = "too_long";

  return Object.fromEntries(
    Object.entries(codes).map(([f, c]) => [f, errorText(f as EnquiryField, c as string)]),
  ) as FieldErrors;
}

export type SubmitResult =
  | { ok: true }
  | { ok: false; kind: "fields"; errors: FieldErrors }
  | { ok: false; kind: "limited" | "challenge" | "failed" };

type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

/**
 * Sends the enquiry. A signed-in visitor's session needs a CSRF token, so on
 * `csrf_failed` it fetches one from /auth/me and retries once.
 */
export async function submitEnquiry(
  values: EnquiryValues,
  fetcher: FetchLike = fetch,
): Promise<SubmitResult> {
  const send = (headers: Record<string, string> = {}) =>
    fetcher("/api/v1/enquiries", {
      method: "POST",
      headers: { "Content-Type": "application/json", ...headers },
      body: JSON.stringify(values),
      credentials: "same-origin",
    });
  const code = async (r: Response) => {
    try {
      return (await r.clone().json()) as { error?: string; fields?: Record<string, string> };
    } catch {
      return {};
    }
  };

  try {
    let res = await send();
    if (res.status === 403 && (await code(res)).error === "csrf_failed") {
      const me = await fetcher("/api/v1/auth/me", { credentials: "same-origin" });
      const token = me.ok ? ((await me.json()) as { csrf_token?: string }).csrf_token : undefined;
      if (token) res = await send({ "X-CSRF-Token": token });
    }
    if (res.status === 202) return { ok: true };
    if (res.status === 429) return { ok: false, kind: "limited" };
    const body = await code(res);
    if (res.status === 422 && body.error === "invalid_enquiry" && body.fields) {
      const errors = Object.fromEntries(
        Object.entries(body.fields).map(([f, c]) => [f, errorText(f as EnquiryField, c)]),
      ) as FieldErrors;
      return { ok: false, kind: "fields", errors };
    }
    if (body.error === "challenge_failed" || body.error === "challenge_unavailable") {
      return { ok: false, kind: "challenge" };
    }
    return { ok: false, kind: "failed" };
  } catch {
    return { ok: false, kind: "failed" };
  }
}
