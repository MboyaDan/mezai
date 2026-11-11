import AxeBuilder from "@axe-core/playwright";
import { expect, request, test, type Browser, type Page } from "@playwright/test";
import { mkdirSync, readFileSync } from "node:fs";
import { apiSignIn, signIn } from "./auth";

// Stage 13a through the real UI (AC-10). Ops sets up a fresh project and an
// invoice for Sample Client Ltd; a contributor works it. The Client Admin then
// sees progress, quality, the workforce by code only, the invoice, the team
// and the activity log. The Client Viewer sees no invoices or team, and the
// other organisation's admin cannot open the project at all.

const port = Number(process.env.PORT ?? 3000);
const base = `http://localhost:${port}`;
const shots = "../docs/screenshots/stage-13a";
const run = Date.now().toString(36);
const projectName = `Portal ${run} (Sample)`;
const invoiceNumber = `MZ-${run.toUpperCase()}`;
// A minimal PDF written for this test.
const pdf = Buffer.from("%PDF-1.4\n% MezAI sample invoice for tests only\n%%EOF\n");

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

/** A contributor answers every task on the project, through the API. */
async function workAs(browser: Browser, who: string, project: string) {
  const ctx = await browser.newContext({ baseURL: base });
  const page = await ctx.newPage();
  await signIn(page, who, /\/c$/);
  const prompt = page.getByRole("heading", { name: "Please review the updated documents" });
  await expect(
    page.getByRole("heading", { name: /^(Welcome, |Please review the updated documents)/ }),
  ).toBeVisible();
  if (await prompt.isVisible()) {
    for (const box of await page.getByRole("checkbox", { name: /I have read and accept/ }).all())
      await box.check();
    await page.getByRole("button", { name: "Accept and continue" }).click();
    await expect(prompt).toBeHidden();
  }
  const { csrf_token: csrf } = await (await page.request.get("/api/v1/auth/me")).json();
  const h = { "X-CSRF-Token": csrf, Origin: base };
  for (let i = 0; i < 10; i++) {
    const next = await page.request.post(`/api/v1/me/projects/${project}/next`, { headers: h, data: {} });
    if (next.status() === 409) break; // no_tasks
    expect(next.ok(), await next.text()).toBeTruthy();
    const task = await next.json();
    const sub = await page.request.post(`/api/v1/me/tasks/${task.id}/submit`, {
      headers: { ...h, "Idempotency-Key": `e2e-${run}-${task.id}` },
      data: { answer: { labels: ["Billing"] }, signals: { time_on_task_ms: 40_000 } },
    });
    expect(sub.ok(), await sub.text()).toBeTruthy();
  }
  await ctx.close();
}

test("13a: the client portal shows its own organisation only", async ({ page, browser }) => {
  test.setTimeout(240_000);

  // Ops work, done as the Super Admin so the suite stays under the per-email
  // sign-in limit for ops@ (10 per 15 minutes). A live k = 1 project with a target of 6, four items, and an invoice.
  const ops = await request.newContext({ baseURL: base });
  const h = await apiSignIn(ops, "superadmin@sample.invalid");
  const orgs = await (await ops.get("/api/v1/organisations")).json();
  const org = orgs.organisations.find((o: { slug: string }) => o.slug === "sample-client-ltd");
  const deadline = new Date(Date.now() + 30 * 86_400_000).toISOString().slice(0, 10);
  const created = await ops.post("/api/v1/projects", {
    headers: h,
    data: {
      org_id: org.id,
      task_type: "classification",
      name: projectName,
      instructions_md: "Choose the team that should answer the message.",
      config: { labels: ["Billing", "Network", "Account", "Other"], multi_label: false },
      overlap_k: 1,
      gold_rate: 0,
      pay_per_task_cents: 1500,
      target_count: 6,
      deadline,
      skills: [],
    },
  });
  expect(created.status(), await created.text()).toBe(201);
  const { id: project } = await created.json();
  const check = await ops.post(`/api/v1/projects/${project}/imports`, {
    headers: h,
    multipart: {
      kind: "items",
      file: {
        name: "portal.csv",
        mimeType: "text/csv",
        // Harmless everyday sentences written for the sample.
        buffer: Buffer.from(
          [
            "external_id,text",
            `${run}-1,Nimelipa bili yangu mara mbili mwezi huu.`,
            `${run}-2,Salio langu la akaunti ni kiasi gani?`,
            `${run}-3,Ninataka risiti ya malipo ya jana.`,
            `${run}-4,Bili ya mwezi huu ni kubwa kuliko kawaida.`,
          ].join("\n"),
        ),
      },
    },
  });
  expect(check.ok(), await check.text()).toBeTruthy();
  const commit = await ops.post(`/api/v1/imports/${(await check.json()).id}/commit`, {
    headers: h,
    data: {},
  });
  expect(commit.ok(), await commit.text()).toBeTruthy();
  for (const action of ["ready", "publish"]) {
    const r = await ops.post(`/api/v1/projects/${project}/status`, { headers: h, data: { action } });
    expect(r.ok(), await r.text()).toBeTruthy();
  }
  const inv = await ops.post("/api/v1/invoices", {
    headers: h,
    multipart: {
      org_id: org.id,
      project_id: project,
      number: invoiceNumber,
      issued_on: new Date().toISOString().slice(0, 10),
      amount_cents: "1250000",
      description: "Pilot work (Sample)",
      file: { name: "invoice.pdf", mimeType: "application/pdf", buffer: pdf },
    },
  });
  expect(inv.status(), await inv.text()).toBe(201);

  // A contributor works the four items; Ops accepts any the audit sample routed to review.
  await workAs(browser, "contributor007@sample.invalid", project);
  const queue = await (await ops.get(`/api/v1/review/items?project_id=${project}`)).json();
  for (const it of queue.items as { item_id: string }[]) {
    const d = await ops.post(`/api/v1/review/items/${it.item_id}/decision`, {
      headers: { ...h, "Idempotency-Key": `e2e-${run}-${it.item_id}` },
      data: { decision: "accept_consensus" },
    });
    expect(d.ok(), await d.text()).toBeTruthy();
  }
  await ops.dispose();

  // Client Admin: home with the project's progress and recent activity.
  await signIn(page, "client.admin@sample.invalid", /\/w$/);
  const nav = page.getByRole("navigation").first();
  for (const item of ["Invoices", "Team", "Activity"])
    await expect(nav.getByRole("link", { name: item })).toBeVisible();
  const row = page.locator('section[aria-labelledby="projects-h"] li').filter({ hasText: projectName });
  await expect(row.getByRole("progressbar", { name: "Final items" })).toHaveAttribute(
    "aria-valuetext",
    "4 of 6",
  );
  await expect(row).toContainText("On track");
  const recent = page.locator('section[aria-labelledby="recent-h"]');
  await expect(recent).toContainText(`MezAI team added invoice ${invoiceNumber}`);
  await axe(page, "client home");
  await shot(page, "client-home");

  // Project: progress and quality, datasets and the workforce by code only.
  await row.getByRole("link", { name: projectName }).click();
  await expect(page.getByRole("heading", { name: "Progress and quality" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Recalculate agreement" })).toHaveCount(0);
  await expect(page.getByText("4 of 6 (67%)")).toBeVisible();
  const workforce = page.locator('section[aria-labelledby="workforce-h"]');
  await expect(workforce.getByRole("cell").first()).toHaveText(/^C-[A-Z][0-9A-Z]{5}$/);
  await expect(workforce).not.toContainText("Sample Contributor");
  await expect(workforce).not.toContainText(/C-0\d{3}/);
  await expect(page.getByRole("heading", { name: "Datasets" })).toBeVisible();
  await axe(page, "client project");
  await shot(page, "client-project");

  // Invoices: the PDF downloads.
  await nav.getByRole("link", { name: "Invoices" }).click();
  await expect(page.getByRole("heading", { name: "Invoices", level: 1 })).toBeVisible();
  const invRow = page.getByRole("row").filter({ hasText: invoiceNumber });
  await expect(invRow).toContainText("KES 12,500.00");
  const [download] = await Promise.all([
    page.waitForEvent("download"),
    invRow.getByRole("link", { name: `PDF of invoice ${invoiceNumber}` }).click(),
  ]);
  expect(
    readFileSync(await download.path())
      .subarray(0, 5)
      .toString(),
  ).toBe("%PDF-");
  await axe(page, "client invoices");
  await shot(page, "client-invoices");

  // Team: invite a colleague, then cancel the invitation.
  await nav.getByRole("link", { name: "Team" }).click();
  await expect(page.getByRole("cell", { name: /Sample Client Viewer/ })).toBeVisible();
  const colleague = `colleague.${run}@sample.invalid`;
  await page.getByLabel("Work email").fill(colleague);
  await page.getByRole("button", { name: "Send invitation" }).click();
  const pending = page.getByRole("row").filter({ hasText: colleague });
  await expect(pending).toBeVisible();
  await axe(page, "client team");
  await shot(page, "client-team");
  await pending.getByRole("button", { name: `Cancel the invitation for ${colleague}` }).click();
  await expect(pending).toHaveCount(0);

  // Activity: the admin's own actions by name, staff as MezAI team.
  await nav.getByRole("link", { name: "Activity" }).click();
  await expect(page.getByText(`Sample Client Admin invited ${colleague}`)).toBeVisible();
  await expect(page.getByText(`Sample Client Admin downloaded invoice ${invoiceNumber}`)).toBeVisible();
  await expect(page.getByText(/MezAI team published · /).first()).toBeVisible();
  await axe(page, "client activity");
  await shot(page, "client-activity");

  // Client Viewer: no invoices or team.
  const viewerCtx = await browser.newContext({ baseURL: base });
  const viewer = await viewerCtx.newPage();
  await signIn(viewer, "client.viewer@sample.invalid", /\/w$/);
  const vnav = viewer.getByRole("navigation").first();
  await expect(vnav.getByRole("link", { name: "Activity" })).toBeVisible();
  await expect(vnav.getByRole("link", { name: "Invoices" })).toHaveCount(0);
  await expect(vnav.getByRole("link", { name: "Team" })).toHaveCount(0);
  await viewer.goto("/w/activity");
  await expect(viewer.getByText(`Sample Client Admin invited ${colleague}`)).toBeVisible();
  await expect(viewer.getByText(`invoice ${invoiceNumber}`)).toHaveCount(0);
  await viewerCtx.close();

  // The other organisation's admin cannot open the project or its figures.
  const otherCtx = await browser.newContext({ baseURL: base });
  const other = await otherCtx.newPage();
  await signIn(other, "other.admin@sample.invalid", /\/w$/);
  await expect(other.getByText(projectName)).toHaveCount(0);
  for (const path of ["", "/progress", "/workforce", "/quality"]) {
    expect((await other.request.get(`/api/v1/projects/${project}${path}`)).status(), path).toBe(404);
  }
  await other.goto(`/w/projects/${project}`);
  await expect(other.getByText(projectName)).toHaveCount(0);
  await other.goto("/w/invoices");
  await expect(other.getByText(invoiceNumber)).toHaveCount(0);
  await other.goto("/w/activity");
  await expect(other.getByText(colleague)).toHaveCount(0);
  await otherCtx.close();
});

test("13a: Ops adds an invoice through the form", async ({ page }) => {
  test.setTimeout(60_000);
  await signIn(page, "superadmin@sample.invalid", /\/w$/);
  await page.getByRole("navigation").first().getByRole("link", { name: "Invoices" }).click();
  await expect(page.getByRole("heading", { name: "Add an invoice" })).toBeVisible();
  const number = `MZ-UI-${run.toUpperCase()}`;
  await page.getByLabel("Organisation").selectOption({ label: "Sample Client Ltd (Sample)" });
  await page.getByLabel("Invoice number").fill(number);
  await page.getByLabel("Amount (KES)").fill("4,800");
  await page
    .getByLabel("Invoice PDF")
    .setInputFiles({ name: "invoice.pdf", mimeType: "application/pdf", buffer: pdf });
  await page.getByRole("button", { name: "Add invoice" }).click();
  const row = page.getByRole("row").filter({ hasText: number });
  await expect(row).toContainText("KES 4,800.00");
  await row.getByRole("button", { name: `Update invoice ${number}` }).click();
  await page.getByLabel("Paid on").fill(new Date(Date.now() + 3 * 3600_000).toISOString().slice(0, 10));
  await page.getByRole("button", { name: "Save" }).click();
  await expect(row).toContainText("Paid");
  await axe(page, "ops invoices");
  await shot(page, "ops-invoices");
});
