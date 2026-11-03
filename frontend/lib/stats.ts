import { cacheLife } from "next/cache";
import { fallbackStats } from "@/content/stats";

export type PublicStat = {
  key: string;
  label: string;
  value: number;
  unit: string;
  source: string;
  source_url: string | null;
  verified_at: string;
};

/**
 * Keeps only records that may be shown: a non-empty source, a valid
 * verification date and a finite value. Anything else is dropped, never
 * rendered (AC-03).
 */
export function publishable(input: unknown): PublicStat[] {
  if (!Array.isArray(input)) return [];
  return input.filter((s): s is PublicStat => {
    if (typeof s !== "object" || s === null) return false;
    const r = s as Record<string, unknown>;
    return (
      typeof r.key === "string" &&
      typeof r.label === "string" &&
      r.label.trim() !== "" &&
      typeof r.value === "number" &&
      Number.isFinite(r.value) &&
      typeof r.unit === "string" &&
      typeof r.source === "string" &&
      r.source.trim() !== "" &&
      typeof r.verified_at === "string" &&
      !Number.isNaN(Date.parse(r.verified_at))
    );
  });
}

type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

/** Fetches published stats; any failure returns the bundled fallback. */
export async function loadStats(
  apiOrigin: string,
  fetchFn: FetchLike = fetch,
  timeoutMs = 1500,
): Promise<PublicStat[]> {
  try {
    const res = await fetchFn(`${apiOrigin}/v1/stats`, { signal: AbortSignal.timeout(timeoutMs) });
    if (!res.ok) return publishable(fallbackStats);
    const body = (await res.json()) as { stats?: unknown };
    return publishable(body.stats);
  } catch {
    return publishable(fallbackStats);
  }
}

/** Cached for the page; the free-tier API may be asleep, so it never blocks rendering. */
export async function getStats(): Promise<PublicStat[]> {
  "use cache";
  cacheLife("hours");
  return loadStats((process.env.API_ORIGIN ?? "http://localhost:8080").replace(/\/$/, ""));
}
