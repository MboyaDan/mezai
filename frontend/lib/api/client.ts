// Browser client for the Go API. Every call goes to /api/* on this origin
// (next.config.ts rewrites it). Types come from lib/api/schema.ts, generated
// from backend/openapi.yaml with `npm run gen:api`.

import type { components } from "./schema";

export type Schemas = components["schemas"];
export type Me = Schemas["Me"];
export type ContributorProfile = Schemas["ContributorProfile"];
export type ProfileInput = Schemas["ContributorProfileInput"];
export type MyTest = Schemas["MyQualificationTest"];
export type Attempt = Schemas["QualificationAttempt"];
export type TaskConfig = Schemas["TaskConfig"];
export type ContributorSummary = Schemas["ContributorSummary"];
export type AttemptSummary = Schemas["AttemptSummary"];
export type AttemptReview = Schemas["AttemptReview"];
export type ContributorStatus = Schemas["ContributorStatus"];
export type Organisation = Schemas["Organisation"];
export type TaskTemplate = Schemas["TaskTemplate"];
export type Project = Schemas["Project"];
export type ProjectInput = Schemas["ProjectInput"];
export type ProjectStatus = Schemas["ProjectStatus"];
export type ProjectSkill = Schemas["ProjectSkill"];
export type QAChecks = Schemas["QAChecks"];
export type ImportJob = Schemas["ImportJob"];
export type GoldItem = Schemas["GoldItem"];
export type ProjectRequest = Schemas["ProjectRequest"];
export type ProjectRequestInput = Schemas["ProjectRequestInput"];
export type QualificationTest = Schemas["QualificationTest"];
export type WorkProject = Schemas["WorkProject"];
export type Task = Schemas["Task"];
export type QAIssue = Schemas["QAIssue"];
export type ContentReport = Schemas["ContentReport"];

/** An error answer from the API, or a network failure (status 0). */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly field?: string,
    readonly detail?: string,
    readonly retryAfter?: number,
    /** The parsed error body, for codes that carry more (for example `not_ready` lists `missing`). */
    readonly body?: Record<string, unknown>,
  ) {
    super(detail ?? code);
    this.name = "ApiError";
  }
}

let csrfToken: string | null = null;

/** Set from login and /v1/auth/me; sent as X-CSRF-Token on state-changing calls. */
export function setCsrfToken(token: string | null) {
  csrfToken = token;
}

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

export async function api<T = void>(
  method: Method,
  path: `/v1/${string}`,
  body?: unknown,
  extra: Record<string, string> = {},
): Promise<T> {
  const headers: Record<string, string> = { ...extra };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  return send<T>(method, path, headers, body === undefined ? undefined : JSON.stringify(body));
}

/** POST a multipart form (file uploads). The browser sets the boundary header. */
export function upload<T>(path: `/v1/${string}`, form: FormData): Promise<T> {
  return send<T>("POST", path, {}, form);
}

async function send<T>(
  method: Method,
  path: string,
  extra: Record<string, string>,
  body: BodyInit | undefined,
): Promise<T> {
  const headers: Record<string, string> = { Accept: "application/json", ...extra };
  if (method !== "GET" && csrfToken) headers["X-CSRF-Token"] = csrfToken;
  let res: Response;
  try {
    res = await fetch(`/api${path}`, {
      method,
      headers,
      body,
      credentials: "same-origin",
      cache: "no-store",
    });
  } catch {
    throw new ApiError(0, "network");
  }
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    const retry = Number(res.headers.get("Retry-After")) || undefined;
    throw new ApiError(
      res.status,
      typeof data?.error === "string" ? data.error : "unknown",
      typeof data?.field === "string" ? data.field : undefined,
      typeof data?.message === "string" ? data.message : undefined,
      retry,
      data && typeof data === "object" ? data : undefined,
    );
  }
  return data as T;
}
