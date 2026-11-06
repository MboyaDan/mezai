import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";
import { fallbackStats } from "./stats";
import { publishable } from "@/lib/stats";

// AC-03 and the honesty rules in CLAUDE.md, checked on the source so CI fails
// before anything renders. tests/e2e/honesty.spec.ts checks the rendered pages.

const root = join(__dirname, "..");
const scanned = ["app", "components", "content", "lib"];

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) return files(p);
    return /\.(ts|tsx)$/.test(name) && !/\.test\.ts$/.test(name) ? [p] : [];
  });
}

const sources = scanned
  .flatMap((d) => files(join(root, d)))
  .map((p) => ({
    path: relative(root, p),
    text: readFileSync(p, "utf8"),
  }));

// Claims we may not make without evidence in docs/content/ (rules 1 and 4).
const forbidden: Array<[string, RegExp]> = [
  ["certification", /\b(SOC ?2|ISO ?\/?(IEC )?2700[01]|HIPAA|PCI[- ]DSS|GDPR[- ](certified|compliant))\b/i],
  ["certified claim", /\b(certified|accredited)\b/i],
  ["client logos", /\b(trusted by|our clients include|as seen in|logo wall|client logos?)\b/i],
  ["testimonial", /\b(testimonials?|what our (clients|customers|contributors) say)\b/i],
  ["award", /\baward[- ]winning\b|\bawards?\b/i],
  ["funding", /\b(backed by|funded by|raised \$|series [ab]\b|seed round)/i],
  ["headcount", /\b\d(?:[\d,]*\d)?\+?\s+(contributors|annotators|clients|languages)\b/i],
];

describe("AC-03: no unearned claims in site source", () => {
  it("scans a meaningful set of files", () => {
    expect(sources.length).toBeGreaterThan(40);
  });

  it.each(forbidden)("no %s", (_name, pattern) => {
    const hits = sources.flatMap(({ path, text }) =>
      text
        .split("\n")
        // Comments may name a rule ("no client logos"); only shipped strings count.
        .flatMap((line, i) =>
          !/^\s*(\/\/|\/\*|\*|\{\/\*)/.test(line) && pattern.test(line)
            ? [`${path}:${i + 1}: ${line.trim()}`]
            : [],
        ),
    );
    expect(hits).toEqual([]);
  });

  it("no quotation markup (testimonials need written permission)", () => {
    const hits = sources.filter(({ text }) => /<blockquote|<q[\s>]/.test(text)).map((s) => s.path);
    expect(hits).toEqual([]);
  });

  it("every bundled fallback stat is publishable (source and verified date)", () => {
    expect(publishable(fallbackStats)).toHaveLength(fallbackStats.length);
  });
});

describe("demonstrations are labelled", () => {
  it("every data-demo container's file carries a label", () => {
    const demos = sources.filter(({ text }) => /\bdata-demo\b/.test(text));
    expect(demos.length).toBeGreaterThanOrEqual(4);
    for (const { path, text } of demos) {
      expect(/Demonstration|Worked example|\.label\b/.test(text), path).toBe(true);
    }
  });
});
