import { expect, test } from "@playwright/test";

// "How MezAI assures quality" (Stage 20a, D-172) is printed to a one-page A4
// PDF from /print/quality. Content changes must not push it onto a second
// page: A4 at 96 dpi is 794 × 1123 CSS pixels.
test("the quality one-pager fits on one A4 page and stays out of search", async ({ page }) => {
  await page.setViewportSize({ width: 794, height: 1123 });
  await page.goto("/print/quality");
  await page.emulateMedia({ media: "print" });
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Many careful judgments.");
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  expect(height, "printed height in CSS pixels").toBeLessThanOrEqual(1123);
  await expect(page.locator('meta[name="robots"]')).toHaveAttribute("content", /noindex/);
  await expect(page.getByText("We hold no security certifications yet")).toBeVisible();
});

test("the quality page links the one-pager PDF, and the file is served", async ({ page, request }) => {
  await page.goto("/quality");
  const link = page.getByRole("link", { name: "Download a one-page summary (PDF)" });
  await expect(link).toHaveAttribute("href", "/downloads/how-mezai-assures-quality.pdf");
  const res = await request.get("/downloads/how-mezai-assures-quality.pdf");
  expect(res.ok()).toBeTruthy();
  expect(res.headers()["content-type"]).toContain("application/pdf");
});
