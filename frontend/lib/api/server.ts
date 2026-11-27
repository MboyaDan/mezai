// Server-side reads from the Go API for signed-in pages (Stage 23, D-165).
// A page's first HTML can then hold the session and its data instead of
// waiting for the browser to fetch them after hydration.
//
// Rules:
// - GET only. Anything that changes state (reserving a task, submitting)
//   stays in the browser with the CSRF token.
// - Only the session cookie is forwarded, with the visitor's address the
//   same way the /api proxy sends it (D-156). Nothing is cached or shared
//   between visitors.
// - Never throws and never blocks for long: on any error or after
//   SERVER_READ_TIMEOUT_MS the caller gets null and the page loads that data
//   in the browser as before (the free-tier API may be waking up).

import { cookies, headers } from "next/headers";
import { cache } from "react";
import type { Me } from "@/lib/api/client";
import { CLIENT_IP_HEADER, PROXY_TOKEN_HEADER } from "@/lib/client-ip";

export const SESSION_COOKIE = "mezai_session";
export const SERVER_READ_TIMEOUT_MS = 2500;

const apiOrigin = () => (process.env.API_ORIGIN ?? "http://localhost:8080").replace(/\/$/, "");

/** Data the server already read, keyed by API path, for the page's hooks. */
export type Preloaded = Record<string, unknown>;

/**
 * GETs one API path as the signed-in visitor. Returns null when there is no
 * session cookie, the API answers with an error, or it is too slow.
 */
async function get(path: `/v1/${string}`): Promise<unknown> {
  const session = (await cookies()).get(SESSION_COOKIE)?.value;
  if (!session) return null;
  const incoming = await headers();
  const h = new Headers({ Accept: "application/json", Cookie: `${SESSION_COOKIE}=${session}` });
  const secret = process.env.PROXY_SHARED_SECRET;
  const ip = incoming.get("x-real-ip")?.trim() || incoming.get("x-forwarded-for")?.split(",")[0]?.trim();
  if (secret && ip) {
    h.set(CLIENT_IP_HEADER, ip);
    h.set(PROXY_TOKEN_HEADER, secret);
  }
  const ua = incoming.get("user-agent");
  if (ua) h.set("User-Agent", ua);
  try {
    const res = await fetch(`${apiOrigin()}${path}`, {
      headers: h,
      cache: "no-store",
      signal: AbortSignal.timeout(SERVER_READ_TIMEOUT_MS),
    });
    if (!res.ok) return null;
    return await res.json();
  } catch {
    return null;
  }
}

/**
 * One read per path per request: a layout and its page may both ask for the
 * same path (Stage 26). Each request starts with an empty cache.
 */
const cachedGet = cache(get);

export function serverGet<T>(path: `/v1/${string}`): Promise<T | null> {
  return cachedGet(path) as Promise<T | null>;
}

/** Reads several paths at once; only the ones that succeeded are kept. */
export async function serverPreload(paths: `/v1/${string}`[]): Promise<Preloaded> {
  const values = await Promise.all(paths.map((p) => serverGet<unknown>(p)));
  const out: Preloaded = {};
  paths.forEach((p, i) => {
    if (values[i] != null) out[p] = values[i];
  });
  return out;
}

/**
 * The visitor's session, read at most once per request: a layout and its
 * page both ask for it (Stage 26).
 */
export const serverMe = () => serverGet<Me>("/v1/auth/me");

/** Whether the server-read session holds any of `keys` (display only; the API checks). */
export function meCan(me: Me | null, ...keys: string[]): boolean {
  const perms = me?.user.permissions ?? [];
  return perms.includes("system.admin") || keys.some((k) => perms.includes(k));
}

/**
 * Whether Google sign-in is switched on in the API (Stage 29, D-171). A
 * public read without the visitor's cookie; false when the API is slow or
 * asleep, so the sign-in page never waits long for it.
 */
export const serverGoogleEnabled = cache(async (): Promise<boolean> => {
  try {
    const res = await fetch(`${apiOrigin()}/v1/auth/providers`, {
      headers: { Accept: "application/json" },
      cache: "no-store",
      signal: AbortSignal.timeout(1500),
    });
    if (!res.ok) return false;
    return Boolean(((await res.json()) as { google?: unknown }).google === true);
  } catch {
    return false;
  }
});
