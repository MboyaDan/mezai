import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// next/headers only works inside a request; stand in for the visitor's.
const jar = new Map<string, string>();
let incoming = new Headers();
vi.mock("next/headers", () => ({
  cookies: async () => ({ get: (n: string) => (jar.has(n) ? { name: n, value: jar.get(n) } : undefined) }),
  headers: async () => incoming,
}));

const { serverGet, serverPreload } = await import("./server");

const fetchMock = vi.fn();
beforeEach(() => {
  jar.clear();
  incoming = new Headers();
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  vi.stubEnv("API_ORIGIN", "http://api.test/");
  vi.stubEnv("PROXY_SHARED_SECRET", "");
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

const ok = (body: unknown) => new Response(JSON.stringify(body), { status: 200 });

describe("serverGet (D-165)", () => {
  it("does not call the API without a session cookie", async () => {
    expect(await serverGet("/v1/auth/me")).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("forwards only the session cookie, uncached, with a timeout", async () => {
    jar.set("mezai_session", "abc");
    jar.set("other", "should-not-travel");
    incoming = new Headers({ cookie: "mezai_session=abc; other=should-not-travel", "user-agent": "UA" });
    fetchMock.mockResolvedValue(ok({ user: { kind: "contributor" } }));
    expect(await serverGet("/v1/auth/me")).toEqual({ user: { kind: "contributor" } });
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("http://api.test/v1/auth/me");
    const h = init.headers as Headers;
    expect(h.get("cookie")).toBe("mezai_session=abc");
    expect(h.get("user-agent")).toBe("UA");
    expect(h.get("x-mezai-client-ip")).toBeNull(); // no secret configured
    expect(init.cache).toBe("no-store");
    expect(init.signal).toBeInstanceOf(AbortSignal);
  });

  it("passes the visitor's address only with the shared secret", async () => {
    vi.stubEnv("PROXY_SHARED_SECRET", "s".repeat(32));
    jar.set("mezai_session", "abc");
    incoming = new Headers({ "x-forwarded-for": "198.51.100.7, 10.0.0.1", "x-mezai-proxy-token": "forged" });
    fetchMock.mockResolvedValue(ok({}));
    await serverGet("/v1/me/contributor");
    const h = fetchMock.mock.calls[0][1].headers as Headers;
    expect(h.get("x-mezai-client-ip")).toBe("198.51.100.7");
    expect(h.get("x-mezai-proxy-token")).toBe("s".repeat(32));
  });

  it("gives null on an error answer, a network failure or a timeout", async () => {
    jar.set("mezai_session", "abc");
    fetchMock.mockResolvedValueOnce(new Response("{}", { status: 401 }));
    expect(await serverGet("/v1/auth/me")).toBeNull();
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));
    expect(await serverGet("/v1/auth/me")).toBeNull();
    fetchMock.mockRejectedValueOnce(new DOMException("timed out", "TimeoutError"));
    expect(await serverGet("/v1/auth/me")).toBeNull();
  });
});

describe("serverPreload", () => {
  it("keeps only the paths that answered", async () => {
    jar.set("mezai_session", "abc");
    fetchMock.mockImplementation(async (url: string) =>
      url.endsWith("/quality") ? new Response("{}", { status: 403 }) : ok({ path: url }),
    );
    const data = await serverPreload(["/v1/me/earnings", "/v1/me/quality"]);
    expect(Object.keys(data)).toEqual(["/v1/me/earnings"]);
  });
});
