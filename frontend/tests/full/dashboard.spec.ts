import AxeBuilder from "@axe-core/playwright";
import { expect, request, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { apiSignIn, signIn } from "./auth";

// Stage 12b through the real UI. A fresh project pays a sample contributor
// for four accepted tasks; the dashboard, Earnings and Training pages then
// show figures that equal the API's, which are the ledger's (D-119), and
// the statement's lines add up to the approved balance. Sample Contributor
// 007 has a seeded 14-day trust history so the score and trend render.

const port = Number(process.env.PORT ?? 3000);
const base = `http://localhost:${port}`;
const shots = "../docs/screenshots/stage-12b";
const run = Date.now().toString(36);
const who = "contributor007@sample.invalid";

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

/** The same text formatKes() renders: "KES 1,234.00". */
const kes = (cents: number) =>
  `KES ${Math.trunc(cents / 100).toLocaleString("en-KE")}.${String(Math.abs(cents) % 100).padStart(2, "0")}`;

/** "+ KES 1,234.00" or "− KES 60.00" → signed cents. */
const cents = (s: string) => {
  const m = s.replace(/\s+/g, " ").match(/([+−-]) KES ([\d,]+)\.(\d{2})/);
  if (!m) throw new Error(`not an amount: ${s}`);
  const v = Number(m[2].replace(/,/g, "")) * 100 + Number(m[3]);
  return m[1] === "+" ? v : -v;
};

test("12b: dashboard, earnings and training match the ledger", async ({ page }) => {
  test.setTimeout(180_000);

  // Ops sets up a live project (k = 1, no gold) with four items.
  const ops = await request.newContext({ baseURL: base });
  const h = await apiSignIn(ops, "ops@sample.invalid");
  const orgs = await (await ops.get("/api/v1/organisations")).json();
  const org = orgs.organisations.find((o: { slug: string }) => o.slug === "sample-client-ltd");
  const created = await ops.post("/api/v1/projects", {
    headers: h,
    data: {
      org_id: org.id,
      task_type: "classification",
      name: `Earnings ${run} (Sample)`,
      instructions_md: "Choose the team that should answer the message.",
      config: { labels: ["Billing", "Network", "Account", "Other"], multi_label: false },
      overlap_k: 1,
      gold_rate: 0,
      pay_per_task_cents: 1500,
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
        name: "earnings.csv",
        mimeType: "text/csv",
        // Harmless everyday sentences written for the sample.
        buffer: Buffer.from(
          [
            "external_id,text",
            `${run}-1,Salio langu la akaunti ni kiasi gani?`,
            `${run}-2,Mtandao haufanyi kazi nyumbani tangu asubuhi.`,
            `${run}-3,Nataka kubadilisha nenosiri la akaunti yangu.`,
            `${run}-4,Nimelipa bili yangu mara mbili mwezi huu.`,
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

  // The contributor signs in once (accepting updated documents if asked).
  await signIn(page, who, /\/c$/);
  await expect(
    page.getByRole("heading", { name: /^(Welcome, |Please review the updated documents)/ }),
  ).toBeVisible();
  const prompt = page.getByRole("heading", { name: "Please review the updated documents" });
  if (await prompt.isVisible()) {
    for (const box of await page.getByRole("checkbox", { name: /I have read and accept/ }).all())
      await box.check();
    await page.getByRole("button", { name: "Accept and continue" }).click();
    await expect(prompt).toBeHidden();
  }

  // Before working, the project is offered with "about 4 tasks left".
  await expect(page.getByRole("heading", { name: "Work available" })).toBeVisible();
  await page.goto("/c/work");
  const workRow = page.getByRole("listitem").filter({ hasText: `Earnings ${run} (Sample)` });
  await expect(workRow).toContainText("KES 15.00");
  await expect(workRow).toContainText("about 4 tasks left");
  await page.goto("/c");
  const row = page.getByRole("listitem").filter({ hasText: `Earnings ${run} (Sample)` });

  // Four tasks through the API.
  const { csrf_token: csrf } = await (await page.request.get("/api/v1/auth/me")).json();
  const ch = { "X-CSRF-Token": csrf, Origin: base };
  for (let i = 0; i < 6; i++) {
    const next = await page.request.post(`/api/v1/me/projects/${project}/next`, { headers: ch, data: {} });
    if (next.status() === 409) break; // no_tasks
    expect(next.ok(), await next.text()).toBeTruthy();
    const task = await next.json();
    const sub = await page.request.post(`/api/v1/me/tasks/${task.id}/submit`, {
      headers: { ...ch, "Idempotency-Key": `e2e-${run}-${task.id}` },
      data: { answer: { labels: ["Billing"] }, signals: { time_on_task_ms: 40_000 } },
    });
    expect(sub.ok(), await sub.text()).toBeTruthy();
  }
  // The audit sample can send an item to review; Ops accepts it.
  const queue = await (await ops.get(`/api/v1/review/items?project_id=${project}`)).json();
  for (const it of queue.items as { item_id: string }[]) {
    const d = await ops.post(`/api/v1/review/items/${it.item_id}/decision`, {
      headers: { ...h, "Idempotency-Key": `e2e-${run}-${it.item_id}` },
      data: { decision: "accept_consensus" },
    });
    expect(d.ok(), await d.text()).toBeTruthy();
  }
  await ops.dispose();

  const summary = await (await page.request.get("/api/v1/me/earnings")).json();
  const kesTotals = summary.totals.find((t: { currency: string }) => t.currency === "KES");
  expect(kesTotals.approved_cents).toBeGreaterThanOrEqual(4 * 1500);

  // Dashboard: the earnings card shows the API's figures; the trust panel
  // shows the seeded score; the trend chart renders.
  await page.goto("/c");
  await expect(page.getByRole("heading", { name: "Trust score" })).toBeVisible();
  const quality = await (await page.request.get("/api/v1/me/quality")).json();
  expect(quality.trust, "seeded trust history").not.toBeNull();
  await expect(page.getByText(quality.trust.score.toFixed(2), { exact: true })).toBeVisible();
  const card = page.getByRole("region", { name: "Earnings" });
  const figure = (label: string, c: number) => new RegExp(`${label}(?:(?!KES).)*${kes(c).replace(/[.]/g, "\\.")}`);
  const figures = card.locator("dl");
  await expect(figures).toContainText(figure("Approved", kesTotals.approved_cents));
  await expect(figures).toContainText(figure("Pending", kesTotals.pending_cents));
  await expect(figures).toContainText(figure("Paid", kesTotals.paid_cents));
  await expect(page.getByRole("figure").filter({ hasText: "Gold accuracy, last 30 days" })).toBeVisible();
  await expect(row, "a project with no work left leaves the dashboard").toHaveCount(0);
  await axe(page, "dashboard");
  await shot(page, "dashboard");

  // Earnings: the statement's lines add up to the approved balance.
  await page.getByRole("link", { name: "Earnings" }).first().click();
  await expect(page.getByRole("heading", { name: "Earnings", level: 1 })).toBeVisible();
  await expect(page.getByText(kes(kesTotals.approved_cents), { exact: true }).first()).toBeVisible();
  const statement = page.locator('section[aria-labelledby="statement-h"] table');
  await expect(statement.getByText(`Earnings ${run} (Sample)`)).toBeVisible();
  while (await page.getByRole("button", { name: "Show older entries" }).isVisible()) {
    await page.getByRole("button", { name: "Show older entries" }).click();
    await page.waitForTimeout(300);
  }
  const amounts = await statement.locator("tbody tr td:last-child").allTextContents();
  expect(amounts.reduce((s, a) => s + cents(a), 0)).toBe(kesTotals.approved_cents);
  await expect(page.getByText("No payouts yet.", { exact: false })).toBeVisible();
  await axe(page, "earnings");
  await shot(page, "earnings");

  // Training: the project's instructions, worked examples and questions.
  await page.getByRole("link", { name: "Training" }).first().click();
  await expect(page.getByRole("heading", { name: "Training", level: 1 })).toBeVisible();
  await expect(page.getByText(`Earnings ${run} (Sample)`).first()).toBeVisible();
  await expect(page.locator("[data-demo]")).toHaveCount(3);
  for (const demo of await page.locator("[data-demo]").all())
    await expect(demo.getByText("Worked example")).toBeVisible();
  await expect(page.getByText(/^trust = 0\.5 × gold accuracy/)).toBeVisible();
  await axe(page, "training");
  await shot(page, "training");
});
