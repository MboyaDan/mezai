import AxeBuilder from "@axe-core/playwright";
import { expect, request as pwRequest, test, type Browser, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";

// Stage 11a through the real UI: two contributors disagree on every item, so
// the items go to review; Ops assigns a reviewer; the reviewer compares the
// answers side by side, chooses one, does not accept the other with a reason
// and moves straight on to the next item. The project is created fresh
// through the API each run.

const seedPassword = process.env.SEED_PASSWORD ?? "sample-password-only";
const port = Number(process.env.PORT ?? 3000);
const base = `http://localhost:${port}`;
const shots = "../docs/screenshots/stage-11a";
const run = Date.now().toString(36);
const projectName = `Review routing ${run} (Sample)`;
const reason = "Instruction 2 says payment messages go to Billing.";

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

async function signIn(page: Page, who: string, home: RegExp) {
  await page.goto("/login");
  await page.getByLabel("Email").filter({ visible: true }).fill(who);
  await page.getByLabel("Password").filter({ visible: true }).fill(seedPassword);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page).toHaveURL(home);
}

async function signOut(page: Page) {
  const menu = page.getByRole("button", { name: /Account menu/ });
  if (await menu.isVisible()) {
    await menu.click();
    await page.getByRole("button", { name: "Sign out" }).click();
  } else {
    await page.getByRole("button", { name: "Sign out" }).first().click();
  }
  await expect(page).toHaveURL(/\/login/, { timeout: 20_000 });
}

/** A contributor signs in (accepting the current documents if asked) and answers every task with one label. */
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
  return { ctx, page, h };
}

test("11a: disagreeing items reach review; a reviewer decides them", async ({ page, browser }) => {
  test.setTimeout(180_000);

  // Setup as Ops through the API: k = 2, so two answers decide each item.
  const api = await pwRequest.newContext({ baseURL: base });
  const login = await api.post("/api/v1/auth/login", {
    headers: { Origin: base },
    data: { email: "ops@sample.invalid", password: seedPassword },
  });
  expect(login.ok(), await login.text()).toBeTruthy();
  const { csrf_token: csrf } = await login.json();
  const h = { "X-CSRF-Token": csrf, Origin: base };
  const orgs = await (await api.get("/api/v1/organisations")).json();
  const org = orgs.organisations.find((o: { slug: string }) => o.slug === "sample-client-ltd");
  const created = await api.post("/api/v1/projects", {
    headers: h,
    data: {
      org_id: org.id,
      task_type: "classification",
      name: projectName,
      instructions_md: "1. Choose the team that should answer.\n2. Payment messages go to Billing.",
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
        name: "review.csv",
        mimeType: "text/csv",
        // Harmless everyday sentences written for the sample.
        buffer: Buffer.from(
          [
            "external_id,text",
            `${run}-1,Nimelipa bili lakini huduma bado haijarudi.`,
            `${run}-2,Malipo yangu ya jana hayajaonekana.`,
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
  await api.dispose();

  // Two contributors disagree on both items: no majority, so both go to review.
  const one = await workAs(browser, "contributor001@sample.invalid", project, "Billing");
  const two = await workAs(browser, "contributor002@sample.invalid", project, "Network");
  await one.ctx.close();

  // Ops assigns the reviewer on the project overview.
  await signIn(page, "ops@sample.invalid", /\/w$/);
  await page.goto(`/w/projects/${project}`);
  await expect(page.getByRole("heading", { name: "Reviewers" })).toBeVisible();
  await page.getByLabel("Add a reviewer").selectOption({ label: "Sample Reviewer" });
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("button", { name: "Remove Sample Reviewer" })).toBeVisible();
  await signOut(page);

  // The reviewer works the queue.
  await signIn(page, "reviewer@sample.invalid", /\/w$/);
  await page.getByRole("link", { name: "Review" }).first().click();
  await expect(page.getByRole("heading", { name: "Review", exact: true })).toBeVisible();
  await page.getByLabel("Project").selectOption({ label: `${projectName} (2)` });
  await expect(page.getByRole("row").filter({ hasText: projectName })).toHaveCount(2);
  await axe(page, "/w/review");
  await shot(page, "review-queue");

  await page
    .getByRole("link", { name: /^Review item/ })
    .first()
    .click();
  await expect(page.getByRole("heading", { name: "Answers (2)" })).toBeVisible();
  await expect(page.getByText("No consensus answer.")).toBeVisible();
  // A Reviewer (no review.adjudicate) cannot write the final answer.
  await expect(page.getByLabel("Write the final answer")).toHaveCount(0);
  await axe(page, "/w/review/[id]");
  await shot(page, "review-item");

  const first = page.url();
  const cards = page.getByRole("listitem").filter({ has: page.getByText(/^Answer \d/) });
  // Match the answer itself: the rejection reason below also mentions Billing.
  const billing = cards.filter({ has: page.getByText("Billing", { exact: true }) });
  const network = cards.filter({ has: page.getByText("Network", { exact: true }) });
  await billing.getByLabel("Use as the final answer").check();
  await network.getByLabel("Do not accept this answer").check();
  await network.getByLabel("Reason for the contributor").fill("short");
  await page.getByRole("button", { name: "Record decision" }).click();
  await expect(page.getByText("Give the contributor a reason (at least 10 characters).")).toBeVisible();
  await network.getByLabel("Reason for the contributor").fill(reason);
  await billing.getByLabel("Feedback").fill("Right call: the message is about a payment.");
  await page.getByRole("button", { name: "Record decision" }).click();

  // Straight on to the second item.
  await expect(page).not.toHaveURL(first);
  await expect(page.getByRole("heading", { name: "Answers (2)" })).toBeVisible();
  await page.getByRole("radio", { name: "Use one contributor's answer" }).check();
  await page.getByRole("button", { name: "Record decision" }).click();
  await expect(page.getByText("Choose the answer to use.")).toBeVisible();
  await billing.getByLabel("Use as the final answer").check();
  await page.getByRole("button", { name: "Record decision" }).click();
  await expect(page.getByText("Chose one answer")).toBeVisible();
  await expect(page.getByText("Accepted").first()).toBeVisible();
  await axe(page, "decided item");
  await shot(page, "review-decided");

  // The queue is empty for the project, and the first decision stands.
  await page.goto(first);
  await expect(page.getByText(`Reason: ${reason}`)).toBeVisible();
  await page.goto(`/w/review?project=${project}`);
  await expect(page.getByText("Nothing waiting with these filters.")).toBeVisible();

  // The contributor whose answer was not accepted sees the reason.
  const fb = await (await two.page.request.get("/api/v1/me/feedback")).json();
  expect(JSON.stringify(fb)).toContain(reason);
  await two.ctx.close();
});
