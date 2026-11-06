import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { legalDocs } from "./legal";

// The API stores the version a contributor accepted and refuses any other
// (backend/internal/legal). The two lists must agree, or sign-up breaks.
const goSource = readFileSync(join(__dirname, "../../backend/internal/legal/legal.go"), "utf8");
const goKeys: Record<string, string> = {
  PrivacyNotice: "privacy",
  TermsOfUse: "terms",
  ContributorTerms: "contributor-terms",
  AcceptableUse: "acceptable-use",
};

describe("legal versions", () => {
  it("match the versions the API accepts", () => {
    const current = [...goSource.matchAll(/^\s+(\w+):\s+"([^"]+)",$/gm)];
    expect(current.length).toBe(Object.keys(goKeys).length);
    for (const [, key, version] of current) {
      const slug = goKeys[key];
      expect(slug, `unknown Go key ${key}`).toBeDefined();
      expect(legalDocs.find((d) => d.slug === slug)?.version, slug).toBe(version);
    }
  });
});
