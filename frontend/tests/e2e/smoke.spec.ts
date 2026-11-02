import { expect, test } from "@playwright/test";

test("home renders and reports API status", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "MezAI" })).toBeVisible();

  const status = page.getByRole("status");
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
