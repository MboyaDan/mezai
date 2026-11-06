import { readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { legalDocs } from "@/content/legal";

// Every document has its own route, and every route has a document.
describe("legal routes", () => {
  it("match content/legal.ts", () => {
    const dir = join(__dirname, "../../app/(public)/legal");
    expect(readdirSync(dir).sort()).toEqual(legalDocs.map((d) => d.slug).sort());
  });
});
