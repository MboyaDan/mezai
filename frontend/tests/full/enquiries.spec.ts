import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { signIn } from "./auth";

// Stage 24 through the real UI: a visitor's enquiry reaches the Ops inbox
// and the home page count; Ops assigns it, notes the call, marks it lost and
// reopens it. A client admin has no inbox. Super Admin signs in for Ops
// because the suite already uses most of ops@'s per-email sign-ins.

const shots = "../docs/screenshots/stage-24";
const origin = `http://localhost:${Number(process.env.PORT ?? 3000)}`;
const run = Date.now().toString(36);
const buyerOrg = `Inbox Buyer ${run} (Sample)`;

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

test("Enquiries inbox: assign, note, close and reopen", async ({ page, request }) => {
  const sent = await request.post("/api/v1/enquiries", {
    headers: { Origin: origin },
    data: {
      name: "Sample Inbox Buyer",
      organisation: buyerOrg,
      email: `inbox.${run}@sample.invalid`,
      project_type: "african-language-data",
      languages: "Kiswahili",
      volume: "under-1k",
      timeline: "1-month",
      message: "Sample enquiry for the inbox test.\nA short pilot first.",
      website: "",
    },
  });
  expect(sent.status(), await sent.text()).toBe(202);

  await page.setViewportSize({ width: 1440, height: 900 });
  await signIn(page, "superadmin@sample.invalid", /\/w$/);

  // The home page count links to the new ones.
  const o = await (await page.request.get("/api/v1/ops/overview")).json();
  const card = page.getByRole("link", { name: /New enquiries to assign/ });
  await expect(card).toContainText(String(o.queues.new_enquiries));
  await card.click();
  await expect(page).toHaveURL(/\/w\/enquiries\?status=new$/);
  await expect(page.getByLabel("Status")).toHaveValue("new");
  await expect(page.getByRole("link", { name: buyerOrg })).toBeVisible();
  await axe(page, "inbox");

  await page.getByRole("link", { name: buyerOrg }).click();
  await expect(page.getByRole("heading", { level: 1, name: buyerOrg })).toBeVisible();
  await expect(page.getByText("A short pilot first.")).toBeVisible();
  await expect(page.getByRole("link", { name: `inbox.${run}@sample.invalid` })).toHaveAttribute(
    "href",
    `mailto:inbox.${run}@sample.invalid`,
  );
  await axe(page, "enquiry new");

  await page
    .getByRole("combobox", { name: /^Assigned to/ })
    .selectOption({ label: "Sample Super Admin (you)" });
  await expect(page.getByRole("button", { name: "Start conversation" })).toBeVisible();
  await page.getByRole("button", { name: "Start conversation" }).click();
  await expect(page.getByRole("button", { name: "Mark won" })).toBeVisible();

  await page.getByLabel("Add a note").fill("Called the buyer. Sending a sample delivery.");
  await page.getByRole("button", { name: "Add note" }).click();
  await expect(page.getByText("Called the buyer.")).toBeVisible();
  await expect(page.getByLabel("Add a note")).toHaveValue("");
  await axe(page, "enquiry in conversation");
  await shot(page, "enquiry");

  await page.getByRole("button", { name: "Mark lost" }).click();
  await expect(page.getByRole("button", { name: "Reopen" })).toBeVisible();
  await expect(page.getByText("Handled by")).toBeVisible();
  await page.getByRole("button", { name: "Reopen" }).click();
  await expect(page.getByRole("button", { name: "Mark won" })).toBeVisible();

  // The won dialog offers active organisations; cancel leaves it open.
  await page.getByRole("button", { name: "Mark won" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByLabel("Organisation")).toBeEnabled();
  await axe(page, "won dialog");
  await shot(page, "enquiry-won-dialog");
  await dialog.getByRole("button", { name: "Cancel" }).click();

  // "Assigned to me" lists it.
  await page.goto("/w/enquiries");
  await page.getByLabel("Assigned to me").check();
  await expect(page.getByRole("link", { name: buyerOrg })).toBeVisible();
  // Screenshot this filtered view: a developer's local inbox may hold real enquiries.
  await shot(page, "enquiries");
});

test("A client admin has no enquiries inbox", async ({ page }) => {
  await signIn(page, "client.admin@sample.invalid", /\/w$/);
  await expect(page.getByRole("link", { name: "Enquiries" })).toHaveCount(0);
  expect((await page.request.get("/api/v1/enquiries")).status()).toBe(403);
});
