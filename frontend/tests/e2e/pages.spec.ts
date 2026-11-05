import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

// Stage 6a public pages: each renders its heading, fits the viewport, passes
// axe and shows no number without a source (AC-03).
const pages: Array<{ path: string; h1: RegExp }> = [
  { path: "/services", h1: /Careful human judgment/ },
  { path: "/quality", h1: /Many careful judgments/ },
  { path: "/industries", h1: /Different stakes/ },
  { path: "/contributors", h1: /Paid for careful work/ },
  { path: "/about", h1: /Meza is Kiswahili for/ },
  { path: "/case-studies", h1: /First pilots under way/ },
];

async function settle(page: Page) {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  );
}

for (const { path, h1 } of pages) {
  test(`${path} renders, fits and passes axe`, async ({ page }) => {
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1, name: h1 })).toBeVisible();
    await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);

    const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
    expect(overflow).toBeLessThanOrEqual(0);

    for (const stat of await page.locator("[data-stat]").all()) {
      await expect(stat).toContainText("Source:");
    }

    await settle(page);
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
      .analyze();
    const blocking = results.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
    expect(blocking.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`)).toEqual([]);
  });
}

test("demonstrations and worked examples are labelled", async ({ page }) => {
  await page.goto("/services");
  await expect(page.getByText("Demonstration", { exact: true }).first()).toBeVisible();
  await page.goto("/quality");
  await expect(page.getByText("manifest.json")).toBeVisible();
  await expect(page.locator("figure", { hasText: "manifest.json" })).toContainText("Demonstration");
  await expect(page.getByText("Worked example").first()).toBeVisible();
});

test("kappa explorer recomputes from the sliders", async ({ page }) => {
  await page.goto("/quality#agreement");
  const slider = page.getByLabel("Messages they label the same");
  await slider.fill("50");
  await expect(page.getByText("100%", { exact: true })).toBeVisible();
  await expect(page.getByText("κ 1.00")).toBeVisible();
});

test("case studies: unpublished slugs are 404 and the nav does not link them", async ({ page }) => {
  const res = await page.goto("/case-studies/__none__");
  expect(res?.status()).toBe(404);
  await page.goto("/");
  await expect(page.locator('header a[href="/case-studies"], footer a[href="/case-studies"]')).toHaveCount(0);
});

test("home links to the security section on /quality", async ({ page }) => {
  await page.goto("/quality#security");
  await expect(page.getByRole("heading", { level: 2, name: "How we handle your data" })).toBeInViewport();
});
