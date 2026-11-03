import { expect, test } from "@playwright/test";

// With motion on, every element GSAP hides must end visible once the page has
// been scrolled, and the scripts must not log errors (DESIGN.md section 9).
test.describe("home motion", () => {
  test.use({ reducedMotion: "no-preference" });

  test("everything revealed after scrolling, no console errors", async ({ page }) => {
    const errors: string[] = [];
    // "Failed to load resource" lines are the browser reporting prefetches of
    // pages Stage 6 has not built yet (A-026); failed assets are caught below.
    page.on(
      "console",
      (m) => m.type() === "error" && !m.text().startsWith("Failed to load resource") && errors.push(m.text()),
    );
    page.on("pageerror", (e) => errors.push(e.message));
    page.on("response", (r) => {
      const kind = r.request().resourceType();
      if (r.status() >= 400 && ["script", "stylesheet", "font", "image"].includes(kind))
        errors.push(`${r.status()} ${r.url()}`);
    });

    await page.goto("/");
    // Wait for GSAP to load (it starts when the browser is idle).
    await page.waitForFunction(
      () =>
        performance.getEntriesByType("resource").some((r) => /gsap|ScrollTrigger/i.test(r.name)) ||
        document.readyState === "complete",
      null,
      { timeout: 10_000 },
    );
    await page.waitForTimeout(2600);

    const height = await page.evaluate(() => document.documentElement.scrollHeight);
    for (let y = 0; y <= height; y += 300) {
      await page.mouse.wheel(0, 300);
      await page.waitForTimeout(60);
    }
    await page.waitForTimeout(1500); // let the last reveals finish

    const hidden = await page.evaluate(() => {
      const targets = [
        ...document.querySelectorAll("[data-reveal]"),
        ...[...document.querySelectorAll("[data-reveal-group]")].flatMap((g) => [...g.children]),
        ...document.querySelectorAll("[data-rule], [data-connector]"),
      ];
      return targets
        .filter((el) => (el as HTMLElement).offsetParent !== null) // ignore display:none variants
        .filter((el) => {
          const cs = getComputedStyle(el);
          return (
            Number(cs.opacity) < 0.99 ||
            (cs.transform !== "none" && cs.transform !== "matrix(1, 0, 0, 1, 0, 0)")
          );
        })
        .map((el) => el.outerHTML.slice(0, 80));
    });
    expect(hidden).toEqual([]);

    // Arcs end fully drawn (no dash offset left over) on the visible map.
    const undrawn = await page.evaluate(
      () =>
        [...document.querySelectorAll<SVGPathElement>("svg:not([class*='hidden']) [data-arc]")]
          .filter((p) => p.getBoundingClientRect().width > 0)
          .filter((p) => {
            const cs = getComputedStyle(p);
            if (cs.strokeDasharray === "none") return false;
            const len = p.getTotalLength();
            const [dash] = cs.strokeDasharray.split(/[ ,]+/).map(parseFloat);
            return dash < len * 0.98;
          }).length,
    );
    expect(undrawn).toBe(0);
    expect(errors).toEqual([]);
  });
});
