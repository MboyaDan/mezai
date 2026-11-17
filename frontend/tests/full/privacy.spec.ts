import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { signIn } from "./auth";

// Stage 17b: a contributor downloads their data and asks for their account
// to be deleted; Ops sees the request, the erase dialog, and closes the
// request (erasure itself is proved in the Go tests, so reruns keep the
// account). Sample Contributor 009 is not used by any other browser test.
const shots = "../docs/screenshots/stage-17b";
const who = "contributor009@sample.invalid";

// Screenshots show the page, not the dev-mode badge or a passing toast.
async function clean(page: Page) {
  await page.addStyleTag({ content: "nextjs-portal { display: none !important; }" });
  await page.mouse.move(0, 0); // a hovered toast stays
  await expect(page.locator('[aria-live="polite"] > [role="status"]')).toHaveCount(0, { timeout: 10_000 });
}

async function settle(page: Page) {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  );
}

async function axe(page: Page, selector: string) {
  await settle(page);
  const r = await new AxeBuilder({ page })
    .include(selector)
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();
  expect(r.violations.filter((v) => v.impact === "critical" || v.impact === "serious")).toEqual([]);
}

test("17b: your data, deletion request and the Ops side", async ({ browser }) => {
  mkdirSync(shots, { recursive: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await signIn(page, who, /\/c/);
  // Start clean if an earlier run left a request open.
  await page.request.delete("/api/v1/me/erasure", {
    headers: { "X-CSRF-Token": (await (await page.request.get("/api/v1/auth/me")).json()).csrf_token },
  });
  await page.goto("/c/profile#your-data");
  const consent = page.getByRole("button", { name: "Accept and continue" });
  await page.getByRole("heading", { level: 1 }).first().waitFor();
  if (await consent.isVisible()) {
    for (const box of await page.getByRole("checkbox", { name: /I have read and accept/ }).all())
      await box.check();
    await consent.click();
    await expect(consent).toHaveCount(0);
    await page.goto("/c/profile#your-data");
  }
  const section = page.getByRole("region", { name: "Your data" });

  // The download is a JSON file about this person only.
  const link = section.getByRole("link", { name: "Download my data" });
  await expect(link).toHaveAttribute("href", "/api/v1/me/data-export");
  const file = await page.request.get("/api/v1/me/data-export");
  expect(file.status()).toBe(200);
  expect(file.headers()["content-disposition"]).toContain("mezai-data-C-");
  const doc = await file.json();
  expect(doc.data.account.email).toBe(who);
  expect(doc.not_included.length).toBeGreaterThan(0);

  await clean(page);
  await section.scrollIntoViewIfNeeded();
  await section.screenshot({ path: `${shots}/your-data-1440.png` });

  await section.getByRole("button", { name: "Ask us to delete my account" }).click();
  const dialog = page.getByRole("dialog", { name: "Ask us to delete your account?" });
  await dialog.getByLabel("Anything we should know").fill("Testing the request (Sample).");
  await dialog.screenshot({ path: `${shots}/delete-request-dialog-1440.png` });
  await dialog.getByRole("button", { name: "Send request" }).click();
  await expect(page.getByText("Request sent to the MezAI team")).toBeVisible();
  await expect(section.getByText(/You asked us to delete your account on/)).toBeVisible();
  await expect(section.getByText("Nothing is outstanding.", { exact: false })).toBeVisible();
  await axe(page, "#your-data");
  await clean(page);
  await section.screenshot({ path: `${shots}/your-data-requested-1440.png` });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await section.scrollIntoViewIfNeeded();
  await section.screenshot({ path: `${shots}/your-data-requested-390.png` });
  const me = await (await page.request.get("/api/v1/me/contributor")).json();
  const contributorId: string = me.id ?? me.profile?.id;
  expect(contributorId).toBeTruthy();
  await ctx.close();

  // Ops: the request is in the queue; the panel shows nothing owed and the
  // erase dialog asks for the code and a reason. Close the request instead.
  const opsCtx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const ops = await opsCtx.newPage();
  await signIn(ops, "ops@sample.invalid", /\/w/);
  await ops.goto("/w/contributors?tab=erasure");
  const row = ops.getByRole("row").filter({ hasText: "In the app" });
  await expect(row).toHaveCount(1);
  await clean(ops);
  await ops.screenshot({ path: `${shots}/ops-deletion-requests-1440.png` });
  await row.getByRole("link").click();
  await expect(ops).toHaveURL(new RegExp(`/w/contributors/${contributorId}`));
  const panel = ops.getByRole("region", { name: "Personal data" });
  await expect(panel.getByText("Deletion requested")).toBeVisible();
  await expect(panel.getByText("Nothing is owed either way.")).toBeVisible();
  await panel.getByRole("button", { name: "Erase personal data" }).click();
  const erase = ops.getByRole("dialog", { name: /^Erase C-/ });
  await erase.getByRole("button", { name: "Erase" }).click();
  await expect(erase.getByText(/Type C-\d+ to confirm\./)).toBeVisible();
  await axe(ops, "dialog[open]");
  await erase.screenshot({ path: `${shots}/ops-erase-dialog-1440.png` });
  await erase.getByRole("button", { name: "Cancel" }).click();
  await panel.getByRole("button", { name: "Close request" }).click();
  await expect(ops.getByText("Request closed")).toBeVisible();
  await clean(ops);
  await panel.scrollIntoViewIfNeeded();
  await panel.screenshot({ path: `${shots}/ops-personal-data-1440.png` });
  await ops.setViewportSize({ width: 390, height: 844 });
  await ops.waitForTimeout(300);
  expect(await ops.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await panel.scrollIntoViewIfNeeded();
  await panel.screenshot({ path: `${shots}/ops-personal-data-390.png` });

  // Every project shows its retention rule.
  await ops.setViewportSize({ width: 1440, height: 900 });
  await ops.goto("/w/projects");
  await ops
    .getByRole("link", { name: /\(Sample\)/ })
    .first()
    .click();
  const card = ops.getByRole("region", { name: "Data retention" });
  await expect(card.getByText(/Task content and answers (are|will be|were) deleted/)).toBeVisible();
  await clean(ops);
  await card.scrollIntoViewIfNeeded();
  await card.screenshot({ path: `${shots}/project-retention-1440.png` });
  await opsCtx.close();
});
