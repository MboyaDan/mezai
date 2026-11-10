import AxeBuilder from "@axe-core/playwright";
import { expect, request as pwRequest, test, type APIRequestContext, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { apiSignIn, signIn } from "./auth";

// Stage 9b through the real UI: a qualified contributor finds a project,
// works on it one task at a time with the keyboard, skips one, reports one
// and runs out of work; Ops resolves the report. The projects are created
// fresh through the API each run so reruns always have work.

const port = Number(process.env.PORT ?? 3000);
const base = `http://localhost:${port}`;
const shots = "../docs/screenshots/stage-09b";
const run = Date.now().toString(36);
const routing = `Message routing ${run} (Sample)`;
const comparing = `Answer comparison ${run} (Sample)`;

async function axe(page: Page, label: string) {
  const r = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  const blocking = r.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
  expect(blocking, `${label}: ${blocking.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`).toEqual([]);
}

async function shot(page: Page, file: string) {
  mkdirSync(shots, { recursive: true });
  // Hide the dev badge and toasts for the picture only; the page keeps running.
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

/** Ops creates a live project with items through the API. */
async function liveProject(
  api: APIRequestContext,
  csrf: string,
  input: Record<string, unknown>,
  file: { name: string; mimeType: string; body: string },
): Promise<string> {
  const h = { "X-CSRF-Token": csrf, Origin: base };
  const orgs = await (await api.get("/api/v1/organisations")).json();
  const org = orgs.organisations.find((o: { slug: string }) => o.slug === "sample-client-ltd");
  const created = await api.post("/api/v1/projects", { headers: h, data: { org_id: org.id, ...input } });
  expect(created.status(), await created.text()).toBe(201);
  const { id } = await created.json();
  const check = await api.post(`/api/v1/projects/${id}/imports`, {
    headers: h,
    multipart: {
      kind: "items",
      file: { name: file.name, mimeType: file.mimeType, buffer: Buffer.from(file.body) },
    },
  });
  expect(check.ok(), await check.text()).toBeTruthy();
  const job = await check.json();
  const commit = await api.post(`/api/v1/imports/${job.id}/commit`, { headers: h, data: {} });
  expect(commit.ok(), await commit.text()).toBeTruthy();
  for (const action of ["ready", "publish"]) {
    const r = await api.post(`/api/v1/projects/${id}/status`, { headers: h, data: { action } });
    expect(r.ok(), await r.text()).toBeTruthy();
  }
  return id;
}

test.describe.configure({ mode: "serial" });

test("9b: work a project one task at a time; Ops resolves a report", async ({ page }) => {
  test.setTimeout(180_000);

  // Setup as Ops, through the API (its own cookie jar).
  const api = await pwRequest.newContext({ baseURL: base });
  const csrf = (await apiSignIn(api, "ops@sample.invalid"))["X-CSRF-Token"];
  const sw = [{ kind: "language", value: "sw", min_proficiency: "fluent" }];
  await liveProject(
    api,
    csrf,
    {
      task_type: "classification",
      name: routing,
      instructions_md:
        "Choose the team that should answer the customer message. Skip any message you cannot read.",
      config: { labels: ["Billing", "Network", "Account", "Other"], multi_label: false },
      overlap_k: 1,
      gold_rate: 0,
      pay_per_task_cents: 1500,
      skills: sw,
    },
    {
      name: "routing.csv",
      mimeType: "text/csv",
      // Harmless everyday sentences written for the sample (CLAUDE.md seed rules).
      body: [
        "external_id,text",
        `${run}-1,Nimelipa bili lakini huduma bado haijarudi.`,
        `${run}-2,Simu yangu haipati mtandao tangu jana.`,
        `${run}-3,Ninataka kubadilisha nenosiri langu.`,
        `${run}-4,Asanteni kwa kunisaidia haraka.`,
      ].join("\n"),
    },
  );
  const pairwiseId = await liveProject(
    api,
    csrf,
    {
      task_type: "pairwise",
      name: comparing,
      instructions_md:
        "Read both answers to the prompt and choose the more helpful one. Score each answer, then explain your choice.",
      config: { criteria: ["accuracy", "helpfulness", "safety", "language_quality"] },
      overlap_k: 1,
      gold_rate: 0,
      pay_per_task_cents: 4000,
      min_rationale_chars: 30,
      skills: [{ kind: "task_type", value: "pairwise" }],
    },
    {
      name: "compare.jsonl",
      mimeType: "application/x-ndjson",
      body: JSON.stringify({
        external_id: `${run}-p1`,
        prompt: "How can I keep my phone battery healthy?",
        response_a: "Avoid letting it run to zero often, keep it out of strong heat, and use a good charger.",
        response_b: "Charge it overnight every night and leave it in the sun so it stays warm.",
      }),
    },
  );
  await api.dispose();

  // 1. The contributor finds the project.
  await signIn(page, "contributor001@sample.invalid", /\/c$/);
  // The seed records no consents, so sample contributors are asked to accept
  // the current legal documents first. Wait until the portal has settled on
  // either the dashboard or that prompt: isVisible() does not wait.
  const review = page.getByRole("heading", { name: "Please review the updated documents" });
  await expect(
    page.getByRole("heading", { name: /^(Welcome, |Please review the updated documents)/ }),
  ).toBeVisible();
  if (await review.isVisible()) {
    for (const box of await page.getByRole("checkbox", { name: /I have read and accept/ }).all())
      await box.check();
    await page.getByRole("button", { name: "Accept and continue" }).click();
    await expect(review).toBeHidden();
  }
  await page.getByRole("link", { name: "Tasks" }).first().click();
  await expect(page.getByRole("heading", { name: "Tasks", exact: true })).toBeVisible();
  const row = page.getByRole("listitem").filter({ hasText: routing });
  await expect(row).toContainText("KES 15.00 a task");
  await axe(page, "/c/work");
  await shot(page, "work-list");
  await row.getByRole("link", { name: /^Brief/ }).click();
  await expect(page.getByRole("heading", { name: routing })).toBeVisible();
  await expect(page.getByText("Choose the team that should answer")).toBeVisible();
  await axe(page, "/c/projects/[id]");
  await shot(page, "project-brief");

  // 2. One task at a time, keyboard first.
  await page.getByRole("link", { name: "Start working" }).click();
  await expect(page.getByRole("heading", { name: routing })).toBeVisible();
  await expect(page.getByText(/Time left: \d+:\d\d/)).toBeVisible(); // the reservation clock
  await axe(page, "task workspace");
  await shot(page, "task-classification");
  await page.keyboard.press("?");
  await expect(page.getByRole("dialog", { name: "Keyboard shortcuts" })).toBeVisible();
  await page.keyboard.press("Escape");
  await page.keyboard.press("1");
  await expect(page.getByRole("radio", { name: /Billing/ })).toBeChecked();
  await expect(page.getByText("Draft saved")).toBeVisible();
  await page.keyboard.press("Control+Enter");
  await expect(page.getByText("Submitted", { exact: true })).toBeVisible();
  await expect(page.getByText(/1 submitted on this project \(1 this session\)/)).toBeVisible();

  // 3. Skip needs a reason; the next task follows.
  await page.getByRole("button", { name: "Skip", exact: true }).click();
  const skip = page.getByRole("dialog", { name: "Skip this task?" });
  await skip.getByRole("button", { name: "Skip task" }).click();
  await expect(skip.getByText("Choose a reason.")).toBeVisible();
  await skip.getByLabel("The item or instructions are unclear").check();
  await axe(page, "skip dialog");
  await skip.getByRole("button", { name: "Skip task" }).click();
  await expect(skip).toBeHidden();

  // 4. Report: the task leaves the queue at once.
  await page.getByRole("button", { name: "Report", exact: true }).click();
  const report = page.getByRole("dialog", { name: "Report this content" });
  await report.getByLabel("It contains someone's personal data").check();
  await report.getByLabel("Note").fill("Looks like it names a real customer.");
  await shot(page, "report-dialog");
  await report.getByRole("button", { name: "Report", exact: true }).click();
  await expect(page.getByText("Reported", { exact: true })).toBeVisible();

  // 5. Last task by mouse, then nothing left.
  await page.getByRole("radio", { name: /Network/ }).check();
  await page.getByRole("button", { name: "Submit", exact: true }).click();
  await expect(page.getByText("No more tasks for you here right now")).toBeVisible();
  await expect(page.getByRole("heading", { level: 1, name: routing })).toBeVisible();
  await expect(page.getByText("You submitted 2 in this session.")).toBeVisible();
  await shot(page, "task-done");

  // 6. Pairwise: side by side, scales, and the rationale minimum shown.
  await page.goto(`/c/work/${pairwiseId}`);
  await expect(page.getByRole("heading", { name: comparing })).toBeVisible();
  await expect(page.getByText("Response A", { exact: true }).first()).toBeVisible();
  await expect(page.getByText(/At least 30 characters \(30 to go\)/)).toBeVisible();
  await axe(page, "pairwise task");
  await shot(page, "task-pairwise");
  await page.getByRole("button", { name: "Instructions" }).click();
  await expect(page.getByRole("dialog", { name: "Instructions" })).toContainText("Checked before you submit");
  await page.keyboard.press("Escape");
  await signOut(page);

  // 7. Ops resolves the report.
  await signIn(page, "ops@sample.invalid", /\/w$/);
  await page.getByRole("link", { name: "Reports" }).first().click();
  const reportRow = page.getByRole("row").filter({ hasText: routing });
  await expect(reportRow).toContainText("It contains someone's personal data");
  await axe(page, "/w/reports");
  await shot(page, "ops-reports");
  await reportRow.getByRole("button", { name: /Resolve/ }).click();
  const resolve = page.getByRole("dialog", { name: "Resolve report" });
  await resolve.getByLabel("Dismiss the report").check();
  await resolve
    .getByLabel("Reason")
    .fill("Checked the item: the name is a common placeholder, not a real person.");
  await resolve.getByRole("button", { name: "Resolve" }).click();
  await expect(page.getByText("Report dismissed")).toBeVisible();
  await expect(page.getByRole("row").filter({ hasText: routing })).toHaveCount(0);
});
