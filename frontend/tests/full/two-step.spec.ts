import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { createHmac } from "node:crypto";
import { mkdirSync } from "node:fs";
import { apiSignIn, signIn } from "./auth";

// Stage 17d: the payout approver turns on two-step sign-in, signs in with a
// code, and an admin resets it from the Staff page (a lost phone). The
// spec leaves the account with two-step off, as it found it.
const shots = "../docs/screenshots/stage-17d";
const who = "approver@sample.invalid";

/** RFC 6238 code from a base32 key, computed here, not by the app. */
function totp(secret: string, at = Date.now()): string {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  let bits = "";
  for (const ch of secret.replace(/\s+/g, "").toUpperCase())
    bits += alphabet.indexOf(ch).toString(2).padStart(5, "0");
  const key = Buffer.from((bits.match(/.{8}/g) ?? []).map((b) => parseInt(b, 2)));
  const msg = Buffer.alloc(8);
  msg.writeBigUInt64BE(BigInt(Math.floor(at / 1000 / 30)));
  const h = createHmac("sha1", key).update(msg).digest();
  const off = h[h.length - 1] & 0x0f;
  return String((h.readUInt32BE(off) & 0x7fffffff) % 1_000_000).padStart(6, "0");
}

async function clean(page: Page) {
  await page.addStyleTag({ content: "nextjs-portal { display: none !important; }" });
  await page.mouse.move(0, 0);
  await expect(page.locator('[aria-live="polite"] > [role="status"]')).toHaveCount(0, { timeout: 10_000 });
}

async function axe(page: Page, selector: string) {
  const r = await new AxeBuilder({ page })
    .include(selector)
    .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
    .analyze();
  expect(r.violations.filter((v) => v.impact === "critical" || v.impact === "serious")).toEqual([]);
}

test("17d: turn on two-step, sign in with a code, reset from Staff", async ({ browser, playwright }) => {
  mkdirSync(shots, { recursive: true });
  // Start clean if an earlier run stopped half-way.
  const admin = await playwright.request.newContext({ baseURL: "http://localhost:3000" });
  const headers = await apiSignIn(admin, "superadmin@sample.invalid");
  const users = (await (await admin.get("/api/v1/users")).json()).users as Array<{
    id: string;
    email: string;
  }>;
  const approverId = users.find((u) => u.email === who)!.id;
  await admin.post(`/api/v1/users/${approverId}/two-step/reset`, {
    headers,
    data: { reason: "Test reset (Sample)" },
  });

  // 1. Turn it on from the account page.
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await signIn(page, who, /\/w/);
  await page.goto("/w/account");
  const card = page.getByRole("region", { name: "Two-step sign-in" });
  await expect(card.getByText("Off", { exact: true })).toBeVisible();
  await card.getByRole("button", { name: "Turn on two-step sign-in" }).click();
  await expect(card.getByRole("img", { name: /QR code/ })).toBeVisible();
  const key = (await card.locator('[aria-label^="Key "]').innerText()).replace(/\s+/g, "");
  expect(key).toMatch(/^[A-Z2-7]{32}$/);
  await clean(page);
  await card.screenshot({ path: `${shots}/setup-1440.png` });
  await card.getByLabel(/Type the 6-digit code/).fill("000000");
  await card.getByRole("button", { name: "Turn on", exact: true }).click();
  await expect(card.getByText("That code did not work.", { exact: false })).toBeVisible();
  await card.getByLabel(/Type the 6-digit code/).fill(totp(key));
  await card.getByRole("button", { name: "Turn on", exact: true }).click();
  const codes = card.getByRole("list", { name: "Recovery codes" }).getByRole("listitem");
  await expect(codes).toHaveCount(10);
  await axe(page, "main");
  await clean(page);
  await card.screenshot({ path: `${shots}/recovery-codes-1440.png` });
  await expect(card.getByText("On", { exact: true })).toBeVisible();
  await expect(card.getByRole("button", { name: "Done" })).toBeDisabled();
  await card.getByLabel("I have saved them").check();
  await card.getByRole("button", { name: "Done" }).click();
  await expect(card.getByText("On", { exact: true })).toBeVisible();
  await expect(card.getByText("10 of 10")).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await clean(page);
  await card.screenshot({ path: `${shots}/account-on-390.png` });
  await ctx.close();

  // 2. Sign in again: the password alone is not enough. Use the next step's
  // code (the current one was spent turning two-step on).
  const ctx2 = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const p2 = await ctx2.newPage();
  await p2.goto("/login");
  await p2.getByLabel("Email").filter({ visible: true }).fill(who);
  await p2
    .getByLabel("Password")
    .filter({ visible: true })
    .fill(process.env.SEED_PASSWORD ?? "sample-password-only");
  await p2.getByRole("button", { name: "Sign in" }).click();
  await expect(p2.getByRole("heading", { name: "Enter your code" })).toBeVisible();
  await expect(p2).toHaveURL(/\/login/);
  await axe(p2, "main");
  await clean(p2);
  await p2.screenshot({ path: `${shots}/sign-in-code-1440.png` });
  await p2.getByLabel("Code").filter({ visible: true }).fill("000000");
  await p2.getByRole("button", { name: "Sign in" }).click();
  await expect(p2.getByText("That code did not work.", { exact: false })).toBeVisible();
  await p2
    .getByLabel("Code")
    .filter({ visible: true })
    .fill(totp(key, Date.now() + 30_000));
  await p2.getByRole("button", { name: "Sign in" }).click();
  await expect(p2).toHaveURL(/\/w/);
  await p2.setViewportSize({ width: 390, height: 844 });
  await ctx2.close();

  // 3. An admin resets it from the Staff page.
  const ctx3 = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const ops = await ctx3.newPage();
  await signIn(ops, "superadmin@sample.invalid", /\/w/);
  await ops.goto("/w/staff");
  const row = ops.getByRole("row").filter({ hasText: who });
  await expect(row.getByText("On", { exact: true })).toBeVisible();
  await clean(ops);
  await ops.screenshot({ path: `${shots}/staff-1440.png` });
  await row.getByRole("button", { name: "Reset two-step" }).click();
  const dialog = ops.getByRole("dialog", { name: /Reset two-step sign-in/ });
  await dialog
    .getByLabel("How you confirmed who is asking")
    .fill("Called the approver on their known number (Sample).");
  await axe(ops, "dialog[open]");
  await dialog.screenshot({ path: `${shots}/reset-dialog-1440.png` });
  await dialog.getByRole("button", { name: "Reset" }).click();
  await expect(row.getByText("Off", { exact: true })).toBeVisible();
  await ops.setViewportSize({ width: 390, height: 844 });
  await ops.waitForTimeout(300);
  expect(await ops.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await clean(ops);
  await ops.screenshot({ path: `${shots}/staff-390.png`, fullPage: true });
  await ctx3.close();
  await admin.dispose();
});
