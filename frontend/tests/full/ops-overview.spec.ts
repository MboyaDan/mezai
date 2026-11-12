import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { signIn } from "./auth";

// Stage 13b through the real UI: the Ops home page shows the queues (each
// count equal to the API's), open projects with their pace, and contributor
// health. A Reviewer sees only their review count. Super Admin signs in for
// Ops because the suite already uses most of ops@'s per-email sign-ins.

const shots = "../docs/screenshots/stage-13b";

async function axe(page: Page, label: string) {
  const r = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  const blocking = r.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
  expect(blocking, `${label}: ${blocking.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`).toEqual([]);
}

async function shot(page: Page, file: string) {
  mkdirSync(shots, { recursive: true });
  const style = await page.addStyleTag({
    content: "nextjs-portal, div[aria-live].fixed > * { display: none !important; }",
  });
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: `${shots}/${file}-1440.png`, fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
    `${file} overflows at 390`,
  ).toBeLessThanOrEqual(390);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: `${shots}/${file}-390.png`, fullPage: true });
  await page.setViewportSize({ width: 1440, height: 900 });
  await style.evaluate((n) => (n as Element).remove());
}

test("Ops overview: queues, projects and contributor health", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await signIn(page, "superadmin@sample.invalid", /\/w$/);

  const res = await page.request.get("/api/v1/ops/overview");
  expect(res.status()).toBe(200);
  const o = await res.json();

  await expect(page.getByRole("heading", { name: "Waiting for the team" })).toBeVisible();
  const queues = page.locator('section[aria-labelledby="queues-h"]');
  const review = queues.getByRole("link", { name: /Items waiting for review/ });
  await expect(review).toContainText(String(o.queues.review));
  await expect(queues.getByRole("link", { name: /Applications to screen/ })).toContainText(
    String(o.queues.applications),
  );
  await expect(queues.getByRole("listitem")).toHaveCount(7);

  const projects = page.locator('section[aria-labelledby="ops-projects-h"]');
  if (o.projects.length > 0) {
    await expect(projects.getByRole("link", { name: o.projects[0].name, exact: true })).toBeVisible();
  }
  const health = page.locator('section[aria-labelledby="health-h"]');
  await expect(health.getByText("Active below the floor")).toBeVisible();

  await axe(page, "ops overview");
  await shot(page, "ops-overview");

  // Dark is a data-theme attribute (globals.css), not the system setting.
  await page.evaluate(() => (document.documentElement.dataset.theme = "dark"));
  await page.waitForTimeout(200);
  await axe(page, "ops overview, dark");
  await page.addStyleTag({ content: "nextjs-portal { display: none !important; }" });
  await page.screenshot({ path: `${shots}/ops-overview-dark-1440.png`, fullPage: true });
  await page.evaluate(() => delete document.documentElement.dataset.theme);

  await review.click();
  await expect(page).toHaveURL(/\/w\/review$/);
});

test("Reviewer home shows their review queue only", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await signIn(page, "reviewer@sample.invalid", /\/w$/);
  await expect(page.getByRole("link", { name: /Items waiting for review in your projects/ })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Waiting for the team" })).toHaveCount(0);
  expect((await page.request.get("/api/v1/ops/overview")).status()).toBe(403);
  await axe(page, "reviewer home");
});
