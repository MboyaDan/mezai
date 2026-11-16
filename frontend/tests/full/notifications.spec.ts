import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { signIn } from "./auth";

// Stage 16: a contributor switches an optional email off on their profile;
// the choice is saved straight away and survives a reload. Sample
// Contributor 004 is not used by any other browser test.
const shots = "../docs/screenshots/stage-16";

test("16: email choices on the profile", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await signIn(page, "contributor004@sample.invalid", /\/c/);
  await page.goto("/c/profile#notifications");
  // Sample accounts accepted older legal documents: accept the current ones first.
  const consent = page.getByRole("button", { name: "Accept and continue" });
  await page.getByRole("heading", { level: 1 }).first().waitFor();
  if (await consent.isVisible()) {
    for (const box of await page.getByRole("checkbox", { name: /I have read and accept/ }).all())
      await box.check();
    await consent.click();
    await expect(consent).toHaveCount(0);
    await page.goto("/c/profile#notifications");
  }
  const section = page.getByRole("region", { name: "Email" });
  const projects = section.getByRole("switch", { name: "New projects" });
  await expect(projects).toHaveAttribute("aria-checked", "true");
  await expect(section.getByText("Payouts sent to your M-Pesa number")).toBeVisible();

  await projects.click();
  await expect(page.getByText("You will not get these emails")).toBeVisible();
  await expect(projects).toHaveAttribute("aria-checked", "false");
  const saved = await (await page.request.get("/api/v1/me/notifications")).json();
  expect(saved).toMatchObject({ project_available: false, feedback_received: true });
  await page.reload();
  await expect(
    page.getByRole("region", { name: "Email" }).getByRole("switch", { name: "New projects" }),
  ).toHaveAttribute("aria-checked", "false");

  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  );
  const r = await new AxeBuilder({ page })
    .include("#notifications")
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();
  expect(r.violations.filter((v) => v.impact === "critical" || v.impact === "serious")).toEqual([]);

  mkdirSync(shots, { recursive: true });
  const region = page.getByRole("region", { name: "Email" });
  await region.scrollIntoViewIfNeeded();
  await region.screenshot({ path: `${shots}/email-choices-1440.png` });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await region.scrollIntoViewIfNeeded();
  await region.screenshot({ path: `${shots}/email-choices-390.png` });

  // Put it back so a rerun starts from the default.
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.getByRole("region", { name: "Email" }).getByRole("switch", { name: "New projects" }).click();
  await expect(page.getByText("You will get these emails")).toBeVisible();
});
