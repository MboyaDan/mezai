import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Browser, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { signIn } from "./auth";

// Stage 14b through the real UI (AC-11): Super Admin previews and prepares a
// batch and cannot approve it; the Payout Approver approves it; Super Admin
// exports the file (its total equals the batch), records one line paid with
// a receipt and the rest failed, and the batch settles. The seed's sample
// balances (C-0006 and C-0008 paid, C-0011 held: name not checked) are dated
// five days back, so a fresh seed always has a batch to make. Super Admin
// prepares because the suite already uses most of ops@'s sign-ins.

const shots = "../docs/screenshots/stage-14b";
const port = Number(process.env.PORT ?? 3000);
const base = `http://localhost:${port}`;

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

async function asApprover(browser: Browser, batchUrl: string, run: (page: Page) => Promise<void>) {
  const ctx = await browser.newContext({ baseURL: base, viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  await signIn(page, "approver@sample.invalid", /\/w$/);
  await page.goto(batchUrl);
  await run(page);
  await ctx.close();
}

test("14b: prepare, approve, export and settle a payout batch", async ({ page, browser }) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await signIn(page, "superadmin@sample.invalid", /\/w$/);
  await page.getByRole("navigation").getByRole("link", { name: "Payouts" }).first().click();
  await expect(page.getByRole("heading", { name: "Payouts", level: 1 })).toBeVisible();

  // A local re-run may find this test's earlier batch settled and nobody left
  // to pay; CI always starts from a fresh seed.
  await page.getByRole("button", { name: "Preview" }).click();
  const toPay = page.getByRole("table", { name: "Contributors in this batch" });
  const nothing = page.getByText("Nobody is due a payout for this period");
  await expect(toPay.or(nothing).first()).toBeVisible();
  test.skip(!process.env.CI && (await nothing.isVisible()), "nothing owed: re-seed to run this again");

  await expect(toPay.getByRole("link", { name: "C-0006" })).toBeVisible();
  await expect(toPay.getByRole("link", { name: "C-0008" })).toBeVisible();
  const held = page.getByRole("table", { name: "Held contributors" });
  await expect(held.getByRole("row", { name: /C-0011/ })).toContainText("M-Pesa registered name not checked");
  await axe(page, "preview");
  await shot(page, "payouts-preview");

  // Prepare and send for approval; the preparer cannot approve.
  await page.getByRole("button", { name: "Create draft batch" }).click();
  await expect(page).toHaveURL(/\/w\/payouts\/[0-9a-f-]{36}$/);
  const batchUrl = new URL(page.url()).pathname;
  const id = batchUrl.split("/").pop()!;
  await expect(page.getByRole("heading", { name: "Check the lines, then send for approval" })).toBeVisible();
  await page.getByRole("button", { name: "Send for approval" }).click();
  await expect(page.getByText("You prepared this batch, so another person")).toBeVisible();
  await expect(page.getByRole("button", { name: "Approve batch" })).toHaveCount(0);
  const own = await page.request.post(`/api/v1/payouts/batches/${id}/approve`, {
    headers: { "X-CSRF-Token": await csrf(page), Origin: base },
  });
  expect(own.status()).toBe(403);

  // A second person approves.
  await asApprover(browser, batchUrl, async (p) => {
    await expect(p.getByRole("heading", { name: "Your approval is needed" })).toBeVisible();
    await axe(p, "approval");
    await shot(p, "payout-approval");
    await p.getByRole("button", { name: "Approve batch" }).click();
    await expect(p.getByText("Batch approved")).toBeVisible();
    await expect(p.getByRole("heading", { name: "Approved" })).toBeVisible();
  });

  // Export: the file's total equals the batch.
  await page.reload();
  await expect(page.getByRole("heading", { name: "Export the payment file" })).toBeVisible();
  await page.getByRole("button", { name: "Export payment file" }).click();
  await expect(page.getByRole("heading", { name: "Pay in M-Pesa, then record each result" })).toBeVisible();
  await expect(page.getByText("File columns not yet confirmed").first()).toBeVisible();
  const batch = await (await page.request.get(`/api/v1/payouts/batches/${id}`)).json();
  const file = await page.request.get(`/api/v1/payouts/batches/${id}/file`);
  expect(file.status()).toBe(200);
  expect(file.headers()["cache-control"]).toBe("no-store");
  const rows = (await file.text()).trim().split("\r\n");
  expect(rows[0]).toBe("Phone Number,Amount,Name,Reference");
  const fileTotal = rows.slice(1).reduce((s, r) => s + Number(r.split(",")[1]) * 100, 0);
  expect(fileTotal).toBe(batch.total_cents);
  expect(rows).toHaveLength(batch.line_count + 1);

  // Results: C-0006 paid with a receipt, every other line failed.
  const lines = page.getByRole("table", { name: "Lines in this batch" });
  await lines.getByRole("button", { name: "Record result for C-0006" }).click();
  let dialog = page.getByRole("dialog");
  // Receipts are unique across batches, so each run uses a new made-up one.
  const receipt = `QK${Date.now().toString(36).toUpperCase().slice(-8)}`;
  await dialog.getByLabel("M-Pesa receipt").fill(receipt.toLowerCase());
  await axe(page, "record result");
  await dialog.getByRole("button", { name: "Record paid" }).click();
  await expect(lines.getByRole("row", { name: /C-0006/ })).toContainText(receipt);
  for (const l of batch.lines as Array<{ public_code: string }>) {
    if (l.public_code === "C-0006") continue;
    await lines.getByRole("button", { name: `Record result for ${l.public_code}` }).click();
    dialog = page.getByRole("dialog");
    await dialog.getByRole("radio", { name: "Failed" }).check();
    await dialog.getByLabel("Reason").fill("Number not registered for M-Pesa (Sample)");
    await dialog.getByRole("button", { name: "Record failed" }).click();
    await expect(lines.getByRole("row", { name: new RegExp(l.public_code) })).toContainText("Failed");
  }
  await expect(page.getByText(/^Settled on /)).toBeVisible();
  await expect(page.getByRole("button", { name: /Record result/ })).toHaveCount(0);
  await axe(page, "settled");
  await shot(page, "payout-settled");

  // The list shows the settled batch; the next payout date can be set.
  await page.getByRole("navigation", { name: "Breadcrumb" }).getByRole("link", { name: "Payouts" }).click();
  const next = new Date(Date.now() + 10 * 86_400_000).toISOString().slice(0, 10);
  await page.getByLabel("Date shown to contributors").fill(next);
  await page.getByRole("button", { name: "Save date" }).click();
  await expect(page.getByText(/Contributors see .* as the next payout date/)).toBeVisible();
  const list = await (await page.request.get("/api/v1/payouts/batches")).json();
  expect(list.next_date).toBe(next);
  expect(list.batches[0]).toMatchObject({ id, status: "settled" });
  await expect(page.getByRole("table", { name: "Payout batches" }).getByRole("row").nth(1)).toContainText(
    "Settled",
  );
  await axe(page, "payouts list");
  await shot(page, "payouts-list");
});

async function csrf(page: Page): Promise<string> {
  const me = await (await page.request.get("/api/v1/auth/me")).json();
  return me.csrf_token;
}
