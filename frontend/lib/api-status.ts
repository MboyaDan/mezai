export type ApiStatus = "checking" | "connected" | "unreachable";

type FetchLike = (input: string, init?: RequestInit) => Promise<Response>;

/**
 * Calls the API health endpoint through the Next.js `/api` rewrite.
 * Any network error, timeout, non-2xx status or unexpected body counts as unreachable.
 */
export async function checkApi(fetchFn: FetchLike = fetch, timeoutMs = 5000): Promise<ApiStatus> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetchFn("/api/healthz", { signal: controller.signal, cache: "no-store" });
    if (!res.ok) return "unreachable";
    const body: unknown = await res.json();
    return isHealthy(body) ? "connected" : "unreachable";
  } catch {
    return "unreachable";
  } finally {
    clearTimeout(timer);
  }
}

function isHealthy(body: unknown): boolean {
  return typeof body === "object" && body !== null && (body as { status?: unknown }).status === "ok";
}

export const statusLabel: Record<ApiStatus, string> = {
  checking: "Checking API…",
  connected: "API connected",
  unreachable: "API unreachable",
};
