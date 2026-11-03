import { expect, test } from "@playwright/test";

test("home renders without the API and shows no unsourced numbers", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: /Human judgment, verified/ })).toBeVisible();
  for (const name of ["What we do", "How quality works"]) {
    await expect(page.getByText(name, { exact: true }).first()).toBeVisible();
  }
  // The product window is labelled as a demonstration at every width.
  const demo = page.locator('section[aria-labelledby="demo-title"]');
  await expect(demo.getByText("Demonstration", { exact: true })).toBeVisible();
  // Every stat on the page names its source (AC-03 starts here).
  const stats = page.locator("[data-stat]");
  for (const stat of await stats.all()) {
    await expect(stat).toContainText("Source:");
  }
});

test("design page reports API status", async ({ page }) => {
  await page.goto("/design");
  const status = page.getByRole("status").filter({ hasText: /API/ });
  // With E2E_EXPECT_API=1 (API running) the page must say connected;
  // otherwise either final state is fine, but it must leave "checking".
  if (process.env.E2E_EXPECT_API === "1") {
    await expect(status).toHaveText("API connected");
  } else {
    await expect(status).toHaveText(/API (connected|unreachable)/);
  }
});

test("no horizontal scroll", async ({ page }) => {
  await page.goto("/");
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
