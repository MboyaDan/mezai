import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";

// Stage 18b: DESIGN.md section 8 and question 7 of the design quality gate,
// checked in the source. Every component, page and content file is read.
const root = join(__dirname, "..");
const dirs = ["app", "components", "content", "lib"];

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) return files(p);
    return /\.(tsx?|css)$/.test(name) && !/\.test\.tsx?$/.test(name) ? [p] : [];
  });
}

const sources = dirs
  .flatMap((d) => files(join(root, d)))
  .map((p) => ({ path: relative(root, p), text: readFileSync(p, "utf8") }));

function hits(re: RegExp, allow: (path: string, line: string) => boolean = () => false) {
  const out: string[] = [];
  for (const { path, text } of sources)
    text.split("\n").forEach((line, i) => {
      if (re.test(line) && !allow(path, line)) out.push(`${path}:${i + 1}: ${line.trim().slice(0, 120)}`);
    });
  return out;
}

describe("design rules in the source (DESIGN.md section 8)", () => {
  it("has one gradient: the faint glow behind the hero lattice", () => {
    expect(
      hits(
        /gradient|bg-linear|bg-radial|bg-conic/,
        (p, l) =>
          (p === "components/home/hero.tsx" &&
            /radial-gradient\(closest-side,rgb\(200_240_49\/0\.07\)/.test(l)) ||
          /The one gradient allowed/.test(l),
      ),
    ).toEqual([]);
  });
  it("uses no purple, violet, indigo or fuchsia", () => {
    expect(hits(/\b(purple|violet|indigo|fuchsia)-\d/)).toEqual([]);
  });
  it("uses no glass effects", () => {
    expect(hits(/backdrop-blur|backdrop-filter/)).toEqual([]);
  });
  it("uses no emoji", () => {
    expect(hits(/[\u{1F300}-\u{1FAFF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u)).toEqual([]);
  });
  it("never says revolutionary, cutting-edge, unlock, empower, harness or seamless", () => {
    expect(hits(/\b(revolutionar\w*|cutting-edge|unlock\w*|empower\w*|harness\w*|seamless\w*)\b/i)).toEqual(
      [],
    );
  });
  it("never sets lime as text, icon or thin line outside a dark context", () => {
    // Lime text is allowed only where the element or the file sits on dark
    // (data-theme="dark" sections, the workspace sidebar and top bars).
    // Each use below was checked to sit on a dark background (Stage 18b);
    // the rendered check in tests/full/sweep.spec.ts tests every page.
    const onDark: Record<string, string> = {
      "app/(public)/contributors/page.tsx": "pay section is data-theme dark",
      "components/home/network.tsx": "network section is data-theme dark",
      "components/shells/public-shell.tsx": "public header is data-theme dark",
      "components/ui/toast.tsx": "toasts render with data-theme dark",
      "components/brand/signal-lattice.tsx":
        "the agreement dot, a decorative lime fill DESIGN.md names as a lime use",
    };
    const found = hits(/\b(text|stroke|border|outline|decoration|fill)-lime\b(?!-ink)/);
    const unreviewed = found.filter(
      (h) => !/data-theme="dark"|dark:/.test(h) && !Object.keys(onDark).some((p) => h.startsWith(p + ":")),
    );
    expect(unreviewed).toEqual([]);
  });
});
