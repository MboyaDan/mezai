import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { fileSize, loadProof, percent, proofFiles } from "./proof";

describe("sample delivery", () => {
  const { manifest, records } = loadProof();

  it("is labelled as a sample everywhere it names a client or project", () => {
    expect(manifest.project.name).toMatch(/\(Sample\)$/);
    expect(manifest.project.organisation).toMatch(/\(Sample\)$/);
  });

  it("has the checksums and sizes the manifest states", () => {
    for (const f of manifest.files) {
      const b = readFileSync(join(process.cwd(), "public", "proof", f.name));
      expect(createHash("sha256").update(b).digest("hex")).toBe(f.sha256);
      expect(b.length).toBe(f.bytes);
      expect(f.records).toBe(records.length);
    }
    expect(manifest.files.map((f) => f.name)).toEqual([proofFiles.jsonl, proofFiles.csv]);
  });

  it("delivers only final items, one record each", () => {
    expect(records).toHaveLength(manifest.items.delivered);
    expect(new Set(records.map((r) => r.item_id)).size).toBe(records.length);
  });

  it("formats figures", () => {
    expect(percent(0.8684)).toBe("86.8%");
    expect(percent(1)).toBe("100%");
    expect(percent(null)).toBe("–");
    expect(fileSize(3853)).toBe("3.8 KB");
    expect(fileSize(900)).toBe("900 B");
  });
});
