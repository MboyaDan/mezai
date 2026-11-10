import { describe, expect, it } from "vitest";
import type { DeliverableFile } from "@/lib/api/client";
import { formatBytes, headlineAgreement, itemsSentence, orderedFiles } from "./deliverables";

describe("deliverables", () => {
  it("formats sizes", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(20 * 1024 * 1024)).toBe("20 MB");
  });

  it("describes what was delivered", () => {
    const base = {
      total: 7,
      delivered: 5,
      consensus: 4,
      adjudicated: 1,
      rejected: 0,
      not_final: 0,
      complete: false,
    };
    expect(itemsSentence({ ...base, rejected: 1, not_final: 1 })).toBe(
      "5 of 7 items delivered; 1 rejected at review and left out; 1 still in progress.",
    );
    expect(itemsSentence({ ...base, total: 5, complete: true })).toBe("5 of 5 items delivered.");
  });

  it("puts kappa first and leaves out per-label figures", () => {
    const s = (statistic: string, label: string | null = null) => ({
      statistic: statistic as "percent",
      label,
      value: 0.5,
      band: null,
      items: 1,
      raters: 2,
    });
    expect(
      headlineAgreement([s("percent"), s("cohen_kappa", "Yes"), s("cohen_kappa")]).map((x) => x.statistic),
    ).toEqual(["cohen_kappa", "percent"]);
  });

  it("orders files", () => {
    const f = (file: "data" | "manifest", format: "jsonl" | "csv" | "json") =>
      ({ id: "x", file, format, name: "", bytes: 0, sha256: "", downloads: 0 }) as DeliverableFile;
    expect(
      orderedFiles([f("data", "csv"), f("manifest", "json"), f("data", "jsonl")]).map((x) => x.format),
    ).toEqual(["jsonl", "csv", "json"]);
  });
});
