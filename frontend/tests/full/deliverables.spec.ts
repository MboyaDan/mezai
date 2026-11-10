import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Browser, type Page } from "@playwright/test";
import { mkdirSync, readFileSync } from "node:fs";
import { signIn } from "./auth";

// Stage 11b through the real UI: two contributors agree on every item, so the
// items become final by consensus; Ops exports a deliverable, checks a file
// and publishes it; the client then sees and downloads it. Before publishing,
// the client sees nothing. The project is created fresh each run.

const port = Number(process.env.PORT ?? 3000);
const base = `http://localhost:${port}`;
const shots = "../docs/screenshots/stage-11b";
const run = Date.now().toString(36);
const projectName = `Delivery ${run} (Sample)`;

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

/** A contributor answers every task on the project with one label, through the API. */
async function workAs(browser: Browser, who: string, project: string, label: string) {
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
      data: { answer: { labels: [label] }, signals: { time_on_task_ms: 45_000 } },
    });
    expect(sub.ok(), await sub.text()).toBeTruthy();
  }
  await ctx.close();
}

test("11b: Ops exports and publishes a deliverable; the client downloads it", async ({ page, browser }) => {
  test.setTimeout(180_000);

  // One Ops session drives the setup and the screens; the client has its own
  // context. Each signs in once, so the per-email login limit is never near.
  await signIn(page, "ops@sample.invalid", /\/w$/);
  const api = page.request;
  const { csrf_token: csrf } = await (await api.get("/api/v1/auth/me")).json();
  const h = { "X-CSRF-Token": csrf, Origin: base };
  const orgs = await (await api.get("/api/v1/organisations")).json();
  const org = orgs.organisations.find((o: { slug: string }) => o.slug === "sample-client-ltd");
  const created = await api.post("/api/v1/projects", {
    headers: h,
    data: {
      org_id: org.id,
      task_type: "classification",
      name: projectName,
      instructions_md: "Choose the team that should answer.",
      config: { labels: ["Billing", "Network", "Account", "Other"], multi_label: false },
      overlap_k: 2,
      gold_rate: 0,
      pay_per_task_cents: 2000,
      skills: [],
    },
  });
  expect(created.status(), await created.text()).toBe(201);
  const { id: project } = await created.json();
  const check = await api.post(`/api/v1/projects/${project}/imports`, {
    headers: h,
    multipart: {
      kind: "items",
      file: {
        name: "delivery.csv",
        mimeType: "text/csv",
        // Harmless everyday sentences written for the sample.
        buffer: Buffer.from(
          [
            "external_id,text",
            `${run}-1,Nimelipa bili ya mwezi huu mara mbili.`,
            `${run}-2,Ningependa kujua salio la akaunti yangu ya malipo.`,
          ].join("\n"),
        ),
      },
    },
  });
  expect(check.ok(), await check.text()).toBeTruthy();
  const commit = await api.post(`/api/v1/imports/${(await check.json()).id}/commit`, {
    headers: h,
    data: {},
  });
  expect(commit.ok(), await commit.text()).toBeTruthy();
  for (const action of ["ready", "publish"]) {
    const r = await api.post(`/api/v1/projects/${project}/status`, { headers: h, data: { action } });
    expect(r.ok(), await r.text()).toBeTruthy();
  }

  // Both agree on both items: final by consensus.
  await workAs(browser, "contributor001@sample.invalid", project, "Billing");
  await workAs(browser, "contributor002@sample.invalid", project, "Billing");

  // The random audit sample (5%) can still send an agreed item to review;
  // Ops accepts the consensus there, as they would in the queue.
  const queue = await (await api.get(`/api/v1/review/items?project_id=${project}`)).json();
  for (const it of queue.items as { item_id: string }[]) {
    const d = await api.post(`/api/v1/review/items/${it.item_id}/decision`, {
      headers: { ...h, "Idempotency-Key": `e2e-${run}-${it.item_id}` },
      data: { decision: "accept_consensus" },
    });
    expect(d.ok(), await d.text()).toBeTruthy();
  }

  // Ops exports version 1 and checks the CSV.
  await page.goto(`/w/projects/${project}`);
  await page.getByRole("link", { name: "Deliverables" }).click();
  await expect(page.getByText("Nothing exported yet").filter({ visible: true })).toBeVisible();
  await page.getByRole("button", { name: "Export final items" }).click();
  await expect(page.getByRole("heading", { name: "Version 1" })).toBeVisible();
  await expect(page.getByText("Not published")).toBeVisible();
  await expect(page.getByText("2 of 2 items delivered.")).toBeVisible();
  const csvDownload = page.waitForEvent("download");
  await page.getByRole("button", { name: "Download Answers (CSV)" }).click();
  const csv = await csvDownload;
  expect(csv.suggestedFilename()).toMatch(/^delivery-.*-sample-v1\.csv$/);
  const csvText = readFileSync(await csv.path(), "utf8");
  expect(csvText.split("\n")[0]).toContain("item_id,external_id,text,labels,decided_by");
  expect(csvText).toContain(`${run}-1`);
  await axe(page, "deliverables (Ops)");
  await shot(page, "deliverables-ops");

  // Not yet published: the client sees nothing.
  const clientCtx = await browser.newContext({ baseURL: base, viewport: { width: 1440, height: 900 } });
  const client = await clientCtx.newPage();
  await signIn(client, "client.admin@sample.invalid", /\/w/);
  await client.goto(`/w/projects/${project}/deliverables`);
  await expect(client.getByText("No deliverables yet").filter({ visible: true })).toBeVisible();

  // Ops publishes after confirming.
  await page.getByRole("button", { name: "Publish to client" }).click();
  await expect(page.getByRole("dialog", { name: "Publish version 1?" })).toBeVisible();
  await page.getByRole("dialog").getByRole("button", { name: "Publish", exact: true }).click();
  await expect(page.getByText("Published", { exact: true }).filter({ visible: true })).toBeVisible();

  // The client downloads the manifest.
  await client.reload();
  await expect(client.getByRole("heading", { name: "Version 1" })).toBeVisible();
  await expect(client.getByRole("button", { name: "Export final items" })).toHaveCount(0);
  const manDownload = client.waitForEvent("download");
  await client.getByRole("button", { name: "Download Manifest (JSON)" }).click();
  const man = JSON.parse(readFileSync(await (await manDownload).path(), "utf8"));
  expect(man.schema).toBe("mezai.delivery-manifest");
  expect(man.items.delivered).toBe(2);
  expect(man.items.complete).toBe(true);
  expect(man.files.map((f: { format: string }) => f.format)).toEqual(["jsonl", "csv"]);
  await axe(client, "deliverables (client)");
  await shot(client, "deliverables-client");
  await clientCtx.close();
});
