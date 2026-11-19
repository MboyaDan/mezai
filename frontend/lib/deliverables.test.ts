import { describe, expect, it } from "vitest";
import type { DeliverableFile } from "@/lib/api/client";
import {
  criterionAgreement,
  formatBytes,
  headlineAgreement,
  itemsSentence,
  orderedFiles,
} from "./deliverables";

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

  it("groups score figures by criterion and leaves classification labels out (D-163)", () => {
    const s = (statistic: string, label: string | null, value = 0.5) => ({
      statistic: statistic as "percent",
      label,
      value,
      band: null,
      items: 4,
      raters: 3,
    });
    const rows = criterionAgreement([
      s("percent", "Yes"),
      s("fleiss_kappa", "Yes"),
      s("percent", "fluency", 0.6),
      s("within_one", "fluency", 0.9),
      s("krippendorff_alpha", "fluency", 0.7),
      s("mean_abs_diff", "fluency", 0.45),
      s("percent", null),
    ]);
    expect(rows).toHaveLength(1);
    expect(rows[0].criterion).toBe("fluency");
    expect([
      rows[0].alpha?.value,
      rows[0].percent?.value,
      rows[0].withinOne?.value,
      rows[0].meanDiff?.value,
    ]).toEqual([0.7, 0.6, 0.9, 0.45]);
  });

  it("orders files", () => {
    const f = (file: "data" | "manifest", format: "jsonl" | "csv" | "json") =>
      ({ id: "x", file, format, name: "", bytes: 0, sha256: "", downloads: 0 }) as DeliverableFile;
    expect(
      orderedFiles([f("data", "csv"), f("manifest", "json"), f("data", "jsonl")]).map((x) => x.format),
    ).toEqual(["jsonl", "csv", "json"]);
  });
});
