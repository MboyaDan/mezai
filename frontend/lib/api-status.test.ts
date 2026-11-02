import { describe, expect, it } from "vitest";
import { checkApi } from "./api-status";

const json = (status: number, body: unknown) => async () =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });

describe("checkApi", () => {
  it("is connected when /api/healthz returns status ok", async () => {
    expect(await checkApi(json(200, { status: "ok" }))).toBe("connected");
  });

  it("calls the rewritten health path", async () => {
    let called = "";
    await checkApi(async (url) => {
      called = url;
      return new Response('{"status":"ok"}');
    });
    expect(called).toBe("/api/healthz");
  });

  it("is unreachable on a non-2xx response", async () => {
    expect(await checkApi(json(502, { status: "ok" }))).toBe("unreachable");
  });

  it("is unreachable on an unexpected body", async () => {
    expect(await checkApi(json(200, { status: "degraded" }))).toBe("unreachable");
    expect(await checkApi(async () => new Response("<html>"))).toBe("unreachable");
  });

  it("is unreachable on a network error", async () => {
    expect(await checkApi(async () => Promise.reject(new TypeError("fetch failed")))).toBe("unreachable");
  });

  it("is unreachable when the request times out", async () => {
    const hang = (_: string, init?: RequestInit) =>
      new Promise<Response>((_, reject) =>
        init?.signal?.addEventListener("abort", () => reject(new Error("aborted"))),
      );
    expect(await checkApi(hang, 20)).toBe("unreachable");
  });
});
