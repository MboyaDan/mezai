import { describe, expect, it } from "vitest";
import { loadStats, publishable } from "./stats";

const ok = {
  key: "k",
  label: "Items reviewed (Sample)",
  value: 12,
  unit: "items",
  source: "Platform records",
  source_url: null,
  verified_at: "2026-11-01T00:00:00Z",
};

describe("publishable", () => {
  it("keeps complete records", () => {
    expect(publishable([ok])).toEqual([ok]);
  });

  it("drops a stat without a source or verification date", () => {
    expect(
      publishable([
        { ...ok, source: "" },
        { ...ok, source: "  " },
      ]),
    ).toEqual([]);
    expect(
      publishable([
        { ...ok, verified_at: "" },
        { ...ok, verified_at: "soon" },
      ]),
    ).toEqual([]);
    const noSource: Record<string, unknown> = { ...ok };
    delete noSource.source;
    expect(publishable([noSource])).toEqual([]);
  });

  it("drops non-numbers and garbage", () => {
    expect(publishable([{ ...ok, value: "12" }, { ...ok, value: Number.NaN }, null, 3])).toEqual([]);
    expect(publishable("nope")).toEqual([]);
  });
});

describe("loadStats", () => {
  const json =
    (body: unknown, status = 200) =>
    async () =>
      new Response(JSON.stringify(body), { status });

  it("uses the API when it answers", async () => {
    expect(await loadStats("http://api", json({ stats: [ok] }))).toEqual([ok]);
  });

  it("filters what the API sends", async () => {
    expect(await loadStats("http://api", json({ stats: [{ ...ok, source: "" }] }))).toEqual([]);
  });

  it("falls back when the API is down or errors", async () => {
    const down = async () => {
      throw new Error("ECONNREFUSED");
    };
    expect(await loadStats("http://api", down)).toEqual([]);
    expect(await loadStats("http://api", json({}, 503))).toEqual([]);
  });
});
