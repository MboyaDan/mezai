import { expect, request as pwRequest, test, type APIRequestContext, type Page } from "@playwright/test";
import { mkdirSync, readFileSync } from "node:fs";
import { demoGold, demoItems, goldJsonl, jsonl, type Pair } from "./content";
import {
  batchReconciliation,
  contributorId,
  contributorState,
  enquiry,
  invitationPath,
  reviewCounts,
  useDemoTrustSetting,
  verificationPath,
} from "./db";

// The pilot demo script (brief section 18), run against a deployed site:
//   cd backend && scripts/staging.sh demo
// Steps 1 to 4 (Stage 19e), 5 to 7 (Stage 19f). Everything it creates is marked (Sample) and stays
// on staging, so a presenter can open it afterwards. Each run uses fresh names.

const base = process.env.DEMO_BASE_URL ?? "http://localhost:3000";
const origin = new URL(base).origin;
const shots = process.env.DEMO_SHOTS ?? "../docs/screenshots/stage-19e";
const seedPassword = process.env.SEED_PASSWORD ?? "sample-password-only";
const run = Date.now().toString(36);
const stamp = String(Date.now()).slice(-8);

const orgName = `Demo Client ${run} (Sample)`;
const projectName = `Assistant answer comparison ${run} (Sample)`;
const enquirer = `demo.buyer.${run}@sample.invalid`;
const joiner = {
  email: `demo.joiner.${run}@sample.invalid`,
  name: `Sample Joiner ${stamp.slice(-4)}`,
  password: "a demo contributor passphrase",
  phone: `07${stamp}`,
  nationalId: `3${stamp.slice(1)}`,
};
// Seeded active contributors with the pairwise skill; 004 answers carelessly.
const careful = [
  "contributor002@sample.invalid",
  "contributor003@sample.invalid",
  "contributor005@sample.invalid",
];
const careless = "contributor004@sample.invalid";
const clientAdmin = { email: `demo.client.${run}@sample.invalid`, password: "a demo client passphrase" };
const careless_rationale = "did not read the other one closely";

let projectId = "";

// ---------------------------------------------------------------- helpers

async function shot(page: Page, file: string) {
  mkdirSync(shots, { recursive: true });
  const style = await page.addStyleTag({
    content: "nextjs-portal, div[aria-live].fixed > * { display: none !important; }",
  });
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: `${shots}/${file}-1440.png`, fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: `${shots}/${file}-390.png`, fullPage: true });
  await page.setViewportSize({ width: 1440, height: 900 });
  await style.evaluate((n) => (n as Element).remove());
}

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
// Sign-in limits (20 a minute per address, 10 per account in 15 minutes):
// wait as long as the API says, up to five minutes, a few times over.
const retryAfter = (h?: string) => (Number(h) > 0 ? Math.min(Number(h) + 1, 300) : 61) * 1000;

async function signIn(page: Page, who: string, password = seedPassword) {
  for (let attempt = 0; ; attempt++) {
    await page.goto("/login");
    await page.getByLabel("Email").filter({ visible: true }).fill(who);
    await page.getByLabel("Password").filter({ visible: true }).fill(password);
    const answer = page.waitForResponse((r) => r.url().endsWith("/api/v1/auth/login"));
    await page.getByRole("button", { name: "Sign in" }).click();
    const res = await answer;
    if (res.status() !== 429 || attempt > 3) return;
    await page.waitForTimeout(retryAfter(res.headers()["retry-after"]));
  }
}

async function signOut(page: Page) {
  const menu = page.getByRole("button", { name: /Account menu/ });
  if (await menu.isVisible()) {
    await menu.click();
    await page.getByRole("button", { name: "Sign out" }).click();
  } else {
    await page.getByRole("button", { name: "Sign out" }).first().click();
  }
  await expect(page).toHaveURL(/\/login/, { timeout: 30_000 });
}

/** A signed-in API client, as the browser would be, through the site's /api. */
async function apiAs(who: string) {
  const api = await pwRequest.newContext({ baseURL: base });
  for (let attempt = 0; ; attempt++) {
    const res = await api.post("/api/v1/auth/login", {
      headers: { Origin: origin },
      data: { email: who, password: seedPassword },
    });
    if (res.status() === 429 && attempt < 4) {
      await wait(retryAfter(res.headers()["retry-after"]));
      continue;
    }
    expect(res.ok(), `${who}: ${await res.text()}`).toBeTruthy();
    const { csrf_token: csrf } = await res.json();
    return { api, headers: { "X-CSRF-Token": csrf as string, Origin: origin } };
  }
}

test.describe.configure({ mode: "serial" });

test.beforeAll(async () => {
  // A free API host may be asleep: wake it before the first step.
  const api = await pwRequest.newContext({ baseURL: base });
  for (let i = 0; i < 20; i++) {
    const r = await api.get("/api/readyz").catch(() => null);
    if (r?.ok()) break;
    await wait(5000);
  }
  await api.dispose();
  useDemoTrustSetting();
});

// ---------------------------------------------------------------- step 1

test("1. A visitor reads the site and sends a project enquiry", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  // Scroll the storyboard section by section, as a visitor would.
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  for (let y = 0; y < height; y += 700) {
    await page.mouse.wheel(0, 700);
    await page.waitForTimeout(250);
  }
  await shot(page, "1-home");

  await page.goto("/quality");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await shot(page, "1-quality");

  await page.goto("/contact");
  await page.getByLabel("Your name").fill("Sample Buyer");
  await page.getByLabel("Organisation").fill(orgName);
  await page.getByLabel("Work email").fill(enquirer);
  await page.getByLabel("Project type").selectOption({ label: "AI evaluation and RLHF" });
  await page.getByLabel("Languages").fill("English and Kiswahili");
  await page.getByLabel("Volume").selectOption({ label: "Under 1,000 items" });
  await page.getByLabel("Timeline").selectOption({ label: "Within a month" });
  await page
    .getByLabel("Message")
    .fill("Sample enquiry from the pilot demo: we want people to compare assistant answers.");
  await page.getByRole("button", { name: "Send enquiry" }).click();
  await expect(page.getByRole("status")).toBeVisible({ timeout: 60_000 });
  await shot(page, "1-enquiry-sent");

  const e = enquiry(enquirer);
  expect(e, "the enquiry is saved").not.toBeNull();
  expect(e!.status).toBe("new");
  expect(e!.notices, "the ops inbox is emailed").toBeGreaterThan(0);
});

// ---------------------------------------------------------------- step 2

test("2. Ops sets up the organisation and a pairwise project with gold", async ({ page }) => {
  test.setTimeout(600_000);
  await signIn(page, "ops@sample.invalid");
  await expect(page).toHaveURL(/\/w$/, { timeout: 60_000 });

  // Ops sees the enquiry in the inbox (Stage 24), takes it on and talks to
  // the buyer. Skipped when step 2 runs without step 1 (no enquiry this run).
  const sent = enquiry(enquirer);
  if (sent) {
    await page.goto("/w/enquiries?status=new");
    await page.getByRole("link", { name: orgName }).click();
    await expect(page.getByRole("heading", { level: 1, name: orgName })).toBeVisible();
    await page
      .getByRole("combobox", { name: /^Assigned to/ })
      .selectOption({ label: "Sample Ops Manager (you)" });
    await expect(page.getByText("Assigned", { exact: true }).first()).toBeVisible();
    await page.getByRole("button", { name: "Start conversation" }).click();
    await expect(page.getByText("In conversation", { exact: true }).first()).toBeVisible();
    await page
      .getByLabel("Add a note")
      .fill("Pilot agreed: pairwise comparison, three people per item, 10 percent gold.");
    await page.getByRole("button", { name: "Add note" }).click();
    await expect(page.getByText("Pilot agreed:")).toBeVisible();
    await shot(page, "2-enquiry");
  }

  await page.goto("/w/organisations");
  await page.getByRole("textbox", { name: "Name", exact: true }).fill(orgName);
  await page.getByRole("button", { name: "Add organisation" }).click();
  await expect(page.getByText(orgName).first()).toBeVisible();
  await shot(page, "2-organisations");

  if (sent) {
    // Won, linked to the organisation just created.
    await page.goto(`/w/enquiries/${sent.id}`);
    await page.getByRole("button", { name: "Mark won" }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Organisation").selectOption({ label: orgName });
    await dialog.getByRole("button", { name: "Mark won" }).click();
    await expect(page.getByText(`Won, linked to ${orgName}`)).toBeVisible();
    expect(enquiry(enquirer)!.status).toBe("won");
  }

  await page.goto("/w/projects/new");
  await page.getByLabel("Client organisation").selectOption({ label: orgName });
  await page.getByLabel("Pairwise comparison").check();
  await page.getByLabel("Project name").fill(projectName);
  await page
    .getByLabel(/^Contributor instructions/)
    .fill(
      "Read the question and both answers. Choose the more helpful and safe answer, score each answer, and explain your choice in a sentence.",
    );
  await page.getByLabel(/^Rubric criteria/).fill("Accuracy\nHelpfulness\nSafety");
  await expect(page.getByLabel(/^Contributors per item/)).toHaveValue("3");
  await expect(page.getByLabel(/^Gold mixed in/)).toHaveValue("10");
  await page.getByLabel(/^Pay per approved task/).fill("40.00");
  await page.getByRole("button", { name: "Add a skill" }).click();
  await page.getByRole("combobox", { name: "Kind", exact: true }).selectOption("task_type");
  await page
    .getByRole("combobox", { name: "Skill", exact: true })
    .selectOption({ label: "Pairwise comparison" });
  await shot(page, "2-project-new");
  await page.getByRole("button", { name: "Create draft project" }).click();
  await expect(page.getByRole("heading", { level: 1, name: projectName })).toBeVisible({
    timeout: 30_000,
  });
  projectId = /\/w\/projects\/([0-9a-f-]{36})/.exec(page.url())![1];

  const tabs = page.getByRole("navigation", { name: "Project" });
  const upload = async (name: string, body: string, rows: number) => {
    await page
      .getByLabel(/^File/)
      .filter({ visible: true })
      .setInputFiles({ name, mimeType: "application/x-ndjson", buffer: Buffer.from(body) });
    await page.getByRole("button", { name: "Check file" }).click();
    await expect(page.getByText(`${name}: every row is valid`).filter({ visible: true })).toBeVisible({
      timeout: 60_000,
    });
    await page.getByRole("button", { name: `Import ${rows} rows` }).click();
  };
  await tabs.getByRole("link", { name: "Items", exact: true }).click();
  await expect(page).toHaveURL(/\/items$/);
  await expect(page.getByRole("heading", { name: "Import items" })).toBeVisible();
  await upload("demo-items.jsonl", jsonl(demoItems(run)), 50);
  await expect(page.getByRole("table", { name: /Uploaded files/ })).toContainText("Imported", {
    timeout: 60_000,
  });
  await shot(page, "2-items");
  await tabs.getByRole("link", { name: "Gold", exact: true }).click();
  await expect(page).toHaveURL(/\/gold$/);
  await expect(page.getByRole("heading", { name: "Import gold items" })).toBeVisible();
  await upload("demo-gold.jsonl", goldJsonl(demoGold(run)), 10);
  await expect(page.getByRole("heading", { name: "Gold set (10 active)" })).toBeVisible({
    timeout: 60_000,
  });
  await shot(page, "2-gold");

  await tabs.getByRole("link", { name: "Overview", exact: true }).click();
  await page.getByRole("button", { name: "Mark ready" }).click();
  await expect(page.getByText("Ready", { exact: true }).filter({ visible: true })).toBeVisible();
  await page.getByRole("button", { name: "Publish" }).click();
  await expect(page.getByText("Live", { exact: true }).filter({ visible: true })).toBeVisible();
  await shot(page, "2-project-live");
  await signOut(page);
});

// ---------------------------------------------------------------- step 3

test("3. A new contributor signs up, qualifies and is approved", async ({ page }) => {
  test.setTimeout(900_000);
  const j = joiner;
  await page.goto("/join");
  await page.getByLabel("Display name").fill(j.name);
  await page.getByLabel("Email").fill(j.email);
  await page.getByLabel("Password").fill(j.password);
  await page.getByLabel(/accept the privacy notice/i).check();
  await page.getByLabel(/accept the contributor terms/i).check();
  await shot(page, "3-join");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Check your email" })).toBeVisible({ timeout: 60_000 });

  await page.goto(verificationPath(j.email));
  await expect(page.getByRole("heading", { name: "Email confirmed" })).toBeVisible({ timeout: 30_000 });

  await signIn(page, j.email, j.password);
  await expect(page).toHaveURL(/\/c$/, { timeout: 60_000 });
  await page.getByRole("link", { name: "Complete profile" }).click();
  await expect(page.getByRole("heading", { name: "Your profile" })).toBeVisible();
  await page.getByLabel("Highest education").selectOption("bachelor");
  await page.getByLabel("Hours a week you can work").fill("15");
  await page.getByRole("combobox", { name: "Language 1" }).selectOption("sw");
  await page.getByRole("combobox", { name: "Level" }).selectOption("native");
  await page.getByLabel("Health").check();
  await page
    .getByLabel("Highest study or training that matters for this work")
    .fill("Sample diploma in journalism");
  await page
    .getByLabel("Relevant experience")
    .fill("Sample: two years writing and editing radio news in Kiswahili.");
  await page
    .getByLabel("Why do you want to do this work?")
    .fill("Sample: I like careful reading and flexible hours.");
  await page.getByRole("textbox", { name: "M-Pesa number", exact: true }).fill(j.phone);
  await page.getByLabel("Enter it again").fill(j.phone);
  await page.getByRole("textbox", { name: "National ID number" }).fill(j.nationalId);
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.getByRole("heading", { name: "Application received" })).toBeVisible({ timeout: 30_000 });
  await shot(page, "3-application-received");
  await signOut(page);

  // Ops shortlists the application.
  await signIn(page, "ops@sample.invalid");
  await expect(page).toHaveURL(/\/w$/, { timeout: 60_000 });
  await page.goto("/w/contributors?tab=applications");
  await page
    .getByRole("row")
    .filter({ hasText: j.name })
    .getByRole("link", { name: /Review/ })
    .click();
  await expect(page.getByRole("heading", { name: "Application", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Shortlist", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Shortlist" }).click();
  await expect(page.getByRole("button", { name: "Shortlist", exact: true })).toHaveCount(0);
  await signOut(page);

  // The contributor takes the core check.
  await signIn(page, j.email, j.password);
  await expect(page).toHaveURL(/\/c$/, { timeout: 60_000 });
  await page.goto("/c/qualify");
  const card = page.getByRole("listitem").filter({ hasText: "MezAI core check (Sample)" });
  await card.getByRole("link", { name: "Start" }).click();
  await page.getByRole("button", { name: "Begin the test" }).click();
  const next = () => page.getByRole("button", { name: "Next", exact: true }).click();
  const q = (i: number) => expect(page.getByRole("heading", { name: `Question ${i} of 7` })).toBeVisible();
  await q(1);
  await page.getByRole("radio", { name: /Complaint/ }).check();
  await next();
  await q(2);
  await page.getByRole("radio", { name: /Question/ }).check();
  await next();
  await q(3);
  await page.getByRole("radio", { name: /Other/ }).check();
  await next();
  await q(4);
  await page.getByRole("radio", { name: /Sentence C/ }).check();
  await next();
  await q(5);
  await page.getByRole("radio", { name: /Sentence B/ }).check();
  await next();
  await q(6);
  for (const side of ["A", "B"])
    for (const c of ["Accuracy", "Helpfulness", "Safety", "Language quality"])
      await page
        .getByRole("group", { name: `${c}, response ${side}` })
        .getByRole("radio", { name: "4" })
        .check();
  await page.getByRole("radio", { name: "Response A", exact: true }).check();
  await page.getByLabel("Why?").fill("Response A gives a safe and correct time; B is simply wrong.");
  await next();
  await q(7);
  await page
    .getByRole("textbox", { name: "Your answer", exact: true })
    .fill(
      Array.from(
        { length: 7 },
        () => "I read the instructions again and check each answer against them before I submit.",
      ).join(" "),
    );
  await expect(page.getByText("All answers saved")).toBeVisible({ timeout: 30_000 });
  await page.getByRole("button", { name: "Review and submit" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("heading", { name: "Submitted" })).toBeVisible({ timeout: 30_000 });
  await shot(page, "3-test-submitted");
  await signOut(page);

  // Ops approves the test, verifies the identity and records the interview.
  await signIn(page, "ops@sample.invalid");
  await expect(page).toHaveURL(/\/w$/, { timeout: 60_000 });
  await page.goto("/w/contributors");
  await page
    .getByRole("row")
    .filter({ hasText: j.name })
    .getByRole("link", { name: /Review/ })
    .click();
  await expect(page.getByRole("heading", { name: "MezAI core check (Sample)" })).toBeVisible();
  await page.getByLabel("Strong").check();
  await page.getByRole("button", { name: "Approve" }).click();
  await expect(page.getByText("Approved", { exact: true })).toBeVisible();
  await page.getByRole("link", { name: /C-\d+ Sample Joiner/ }).click();
  await page.getByRole("button", { name: "Verify identity" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel(/Note/).fill("Demo: numbers checked; no other account found.");
  await dialog.getByRole("button", { name: "Verify", exact: true }).click();
  await expect(page.getByRole("button", { name: "Verify identity" })).toHaveCount(0);
  await page.getByRole("button", { name: "Record interview" }).click();
  const iv = page.getByRole("dialog", { name: /Interview: C-/ });
  await iv.getByLabel("Name and ID digits match").check();
  await iv.getByRole("group", { name: "Kiswahili" }).getByLabel("4 out of 4").check();
  await iv.getByRole("group", { name: "Scenario score" }).getByLabel("3 out of 4").check();
  await iv.getByLabel("They keep task content private").check();
  await iv.getByLabel("Hours, device and connection").fill("15 hours a week, smartphone, good signal");
  await iv.getByRole("button", { name: "Save interview" }).click();
  await expect(page.getByText(/Interview passed · /)).toBeVisible();
  await expect(page.getByText("qualifying → ")).toBeVisible();
  await shot(page, "3-ops-approved");
  await signOut(page);

  await signIn(page, j.email, j.password);
  await expect(page.getByText("Active", { exact: true })).toBeVisible({ timeout: 60_000 });
  await shot(page, "3-contributor-active");
  await signOut(page);
});

// ---------------------------------------------------------------- step 4

type Worker = {
  who: string;
  api: APIRequestContext;
  headers: Record<string, string>;
  last: number;
  done: boolean;
};

test("4. Contributors work the project; a careless one is paused automatically", async ({ page }) => {
  test.setTimeout(900_000);
  expect(projectId, "step 2 made the project").not.toBe("");
  const pairs = new Map<string, Pair>();
  for (const p of [...demoItems(run), ...demoGold(run)]) pairs.set(p.prompt + "|" + p.response_a, p);

  // On a rerun the careless contributor is still paused from last time: Ops
  // reinstates them first, as Ops would after a talk.
  const badId = contributorId(careless);
  if (contributorState(careless).status === "paused") {
    const ops = await apiAs("ops@sample.invalid");
    const r = await ops.api.post(`/api/v1/contributors/${badId}/status`, {
      headers: ops.headers,
      data: { status: "active", reason: "Demo rerun: reinstated to show the automatic pause again." },
    });
    expect(r.ok(), await r.text()).toBeTruthy();
    await ops.api.dispose();
  }

  const workers: Worker[] = [];
  for (const who of [...careful, careless])
    workers.push({ who, ...(await apiAs(who)), last: 0, done: false });

  const project = await (await workers[0].api.get(`/api/v1/me/projects/${projectId}`)).json();
  const criteria: string[] = project.config?.criteria ?? project.project?.config?.criteria;
  expect(criteria?.length, JSON.stringify(project).slice(0, 300)).toBeGreaterThan(0);

  const scores = (high: boolean) => Object.fromEntries(criteria.map((c) => [c, high ? 5 : 2]));
  // Careful people prefer the helpful answer; contributor005 disagrees on
  // every tenth item (a spread of 3 sends it to review); 004 always picks
  // the weak one, gold included.
  const answer = (w: Worker, p: Pair) => {
    const n = Number(/-i(\d+)-/.exec(p.external_id)?.[1] ?? -1);
    const dissent = w.who === careful[2] && n >= 0 && n % 10 === 3;
    const pick = w.who === careless || dissent ? (p.better === "a" ? "b" : "a") : p.better;
    return {
      preference: pick,
      scores_a: scores(pick === "a"),
      scores_b: scores(pick === "b"),
      rationale:
        pick === p.better
          ? "The chosen answer is practical, correct and safe; the other is unhelpful or risky."
          : `The chosen answer is shorter and I ${careless_rationale} enough.`,
    };
  };

  let submitted = 0;
  while (workers.some((w) => !w.done)) {
    for (const w of workers.filter((x) => !x.done)) {
      // Stay under the API's 60 tasks a minute for each person.
      const gap = 1100 - (Date.now() - w.last);
      if (gap > 0) await wait(gap);
      w.last = Date.now();
      const next = await w.api.post(`/api/v1/me/projects/${projectId}/next`, { headers: w.headers });
      if (!next.ok()) {
        const body = await next.json().catch(() => ({}));
        expect(["no_tasks", "not_eligible"], `${w.who}: ${next.status()} ${JSON.stringify(body)}`).toContain(
          body.error,
        );
        w.done = true;
        continue;
      }
      const task = await next.json();
      const c = task.content;
      const p = pairs.get(c.prompt + "|" + c.response_a);
      expect(p, `unknown item ${c.prompt}`).toBeDefined();
      const res = await w.api.post(`/api/v1/me/tasks/${task.id}/submit`, {
        headers: { ...w.headers, "Idempotency-Key": `demo-${run}-${task.id}` },
        data: { answer: answer(w, p!), signals: { time_on_task_ms: 45_000, pasted_chars: 0 } },
      });
      expect(res.ok(), `${w.who} submit: ${await res.text()}`).toBeTruthy();
      submitted++;
    }
  }
  console.log(`demo: ${submitted} submissions`);

  const bad = contributorState(careless);
  expect(bad.status, "the careless contributor is paused").toBe("paused");
  expect(bad.score ?? 1).toBeLessThan(0.7);
  for (const who of careful) expect(contributorState(who).status, who).toBe("active");

  // Ops sees the pause with its reason.
  await signIn(page, "ops@sample.invalid");
  await expect(page).toHaveURL(/\/w$/, { timeout: 60_000 });
  await page.goto(`/w/contributors/${badId}`);
  await expect(page.getByText(/below the floor/).first()).toBeVisible();
  await shot(page, "4-ops-paused");
  await signOut(page);
});

// ---------------------------------------------------------------- step 5

test("5. Disagreements reach review; a reviewer decides one and sends feedback", async ({
  page,
  browser,
}) => {
  test.setTimeout(600_000);
  expect(projectId, "step 2 made the project").not.toBe("");
  // Ops gives the project a reviewer.
  await signIn(page, "ops@sample.invalid");
  await expect(page).toHaveURL(/\/w$/, { timeout: 60_000 });
  await page.goto(`/w/projects/${projectId}`);
  await page.getByLabel("Add a reviewer").selectOption({ label: "Sample Reviewer" });
  await page.getByRole("button", { name: "Add", exact: true }).click();
  await expect(page.getByRole("button", { name: "Remove Sample Reviewer" })).toBeVisible();
  await signOut(page);

  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const rv = await ctx.newPage();
  await signIn(rv, "reviewer@sample.invalid");
  await expect(rv).toHaveURL(/\/w$/, { timeout: 60_000 });
  await rv.goto(`/w/review?project=${projectId}`);
  const open = rv.getByRole("link", { name: /^Review item/ });
  await expect(open.first()).toBeVisible({ timeout: 60_000 });
  console.log(`demo: ${await open.count()} items waiting for review`);
  await shot(rv, "5-review-queue");

  await open.first().click();
  await expect(rv.getByRole("heading", { name: /^Answers \(\d\)$/ })).toBeVisible();
  const cards = rv.getByRole("listitem").filter({ has: rv.getByText(/^Answer \d/) });
  const weak = cards.filter({ hasText: careless_rationale });
  const sound = cards.filter({ hasNotText: careless_rationale });
  const nWeak = await weak.count();
  expect(nWeak, "the item has a careless answer").toBeGreaterThan(0);
  await sound.first().getByLabel("Use as the final answer").check();
  for (let i = 0; i < nWeak; i++) {
    const c = weak.nth(i);
    await c.getByLabel("Do not accept this answer").check();
    await c
      .getByLabel("Reason for the contributor")
      .fill("You chose the weaker answer: it is unsafe or unhelpful. Read both answers in full.");
    await c
      .getByLabel("Feedback")
      .fill("Compare both answers against the rubric before choosing; the guidelines show examples.");
  }
  await shot(rv, "5-review-item");
  const before = rv.url();
  await rv.getByRole("button", { name: "Record decision" }).click();
  await expect(rv).not.toHaveURL(before, { timeout: 30_000 });
  await ctx.close();

  const counts = reviewCounts(projectId);
  expect(counts.reviews).toBeGreaterThan(0);
  expect(counts.feedback).toBeGreaterThan(0);
});

// ---------------------------------------------------------------- step 6

test("6. The client signs in, sees progress and quality, and downloads the delivery", async ({
  page,
  browser,
}) => {
  test.setTimeout(600_000);
  expect(projectId, "step 2 made the project").not.toBe("");
  // Ops invites the client, refreshes agreement, exports and publishes.
  await signIn(page, "ops@sample.invalid");
  await expect(page).toHaveURL(/\/w$/, { timeout: 60_000 });
  await page.goto("/w/organisations");
  const invite = page.getByRole("region", { name: "Invite a client user" });
  await invite.getByLabel("Work email").fill(clientAdmin.email);
  await invite.getByLabel("Organisation").selectOption({ label: orgName });
  await invite.getByLabel("Role").selectOption("client_admin");
  await invite.getByRole("button", { name: "Send invitation" }).click();
  await expect(invite.getByRole("row").filter({ hasText: clientAdmin.email })).toBeVisible();

  await page.goto(`/w/projects/${projectId}`);
  await page.getByRole("button", { name: "Recalculate agreement" }).click();
  await page.waitForTimeout(2000);
  await page.getByRole("navigation", { name: "Project" }).getByRole("link", { name: "Deliverables" }).click();
  await page.getByRole("button", { name: "Export final items" }).click();
  await expect(page.getByRole("heading", { name: /^Version \d+$/ }).first()).toBeVisible({ timeout: 60_000 });
  await page.getByRole("button", { name: "Publish to client" }).first().click();
  await page.getByRole("dialog").getByRole("button", { name: "Publish", exact: true }).click();
  await expect(page.getByText("Published", { exact: true }).filter({ visible: true }).first()).toBeVisible();
  await signOut(page);

  // The client accepts the invitation and signs in.
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const cl = await ctx.newPage();
  await cl.goto(invitationPath(clientAdmin.email));
  await cl.getByLabel("Your name").filter({ visible: true }).fill("Sample Client Buyer");
  await cl.getByLabel("Password").filter({ visible: true }).fill(clientAdmin.password);
  await cl.getByRole("button", { name: "Create account" }).click();
  await expect(cl.getByRole("heading", { name: "Your account is ready" })).toBeVisible({ timeout: 30_000 });
  await signIn(cl, clientAdmin.email, clientAdmin.password);
  await expect(cl).toHaveURL(/\/w$/, { timeout: 60_000 });
  await shot(cl, "6-client-home");

  await cl.goto(`/w/projects/${projectId}`);
  await expect(cl.getByRole("heading", { name: "Progress and quality" })).toBeVisible();
  await expect(cl.getByText("Gold accuracy").first()).toBeVisible();
  await expect(cl.getByText("Agreement").first()).toBeVisible();
  await shot(cl, "6-client-project");

  await cl.goto(`/w/projects/${projectId}/deliverables`);
  const manifestFile = cl.waitForEvent("download");
  await cl.getByRole("button", { name: "Download Manifest (JSON)" }).first().click();
  const manifest = JSON.parse(readFileSync(await (await manifestFile).path(), "utf8"));
  expect(manifest.schema).toBe("mezai.delivery-manifest");
  expect(manifest.items.delivered).toBeGreaterThan(0);
  const answers = cl.waitForEvent("download");
  await cl
    .getByRole("button", { name: /^Download Answers \(JSONL\)$/ })
    .first()
    .click();
  const lines = readFileSync(await (await answers).path(), "utf8")
    .trim()
    .split("\n");
  expect(lines.length).toBe(manifest.items.delivered);
  console.log(
    `demo: delivered ${manifest.items.delivered} items; manifest keys ${Object.keys(manifest).join(", ")}`,
  );
  await shot(cl, "6-client-deliverables");
  await ctx.close();
});

// ---------------------------------------------------------------- step 7

test("7. Ops pays what is due; a second person approves; balances update", async ({ page, browser }) => {
  test.setTimeout(600_000);
  await signIn(page, "ops@sample.invalid");
  await expect(page).toHaveURL(/\/w$/, { timeout: 60_000 });
  await page.goto("/w/payouts");
  // A batch covers whole days up to yesterday (Nairobi): today's approvals
  // are paid in the next batch.
  const yesterday = new Date(Date.now() + 3 * 3_600_000 - 86_400_000).toISOString().slice(0, 10);
  await page.getByLabel("Period ends on").fill(yesterday);
  await page.getByRole("button", { name: "Preview" }).click();
  const toPay = page.getByRole("table", { name: "Contributors in this batch" });
  const nothing = page.getByText("Nobody is due a payout for this period");
  const refused = page.getByText(/already covers this day|must end before today/).first();
  await expect(toPay.or(nothing).or(refused).first()).toBeVisible({ timeout: 60_000 });
  if (!(await toPay.isVisible())) {
    await shot(page, "7-payouts-nothing-due");
    test.skip(
      true,
      `nothing to pay up to ${yesterday}: ${await page.locator("main").innerText()}`.slice(0, 300),
    );
  }
  await shot(page, "7-payout-preview");
  await page.getByRole("button", { name: "Create draft batch" }).click();
  await expect(page).toHaveURL(/\/w\/payouts\/[0-9a-f-]{36}$/);
  const batchId = page.url().split("/").pop()!;
  await page.getByRole("button", { name: "Send for approval" }).click();
  await expect(page.getByText("You prepared this batch, so another person")).toBeVisible();

  // The two-person rule: the approver is someone else.
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const ap = await ctx.newPage();
  await signIn(ap, "approver@sample.invalid");
  await expect(ap).toHaveURL(/\/w$/, { timeout: 60_000 });
  await ap.goto(`/w/payouts/${batchId}`);
  await expect(ap.getByRole("heading", { name: "Your approval is needed" })).toBeVisible();
  await shot(ap, "7-payout-approval");
  await ap.getByRole("button", { name: "Approve batch" }).click();
  await expect(ap.getByRole("heading", { name: "Approved" })).toBeVisible();
  await ctx.close();

  await page.reload();
  await page.getByRole("button", { name: "Export payment file" }).click();
  await expect(page.getByRole("heading", { name: "Pay in M-Pesa, then record each result" })).toBeVisible();
  const batch = await (await page.request.get(`/api/v1/payouts/batches/${batchId}`)).json();
  const file = await page.request.get(`/api/v1/payouts/batches/${batchId}/file`);
  const rows = (await file.text()).trim().split("\r\n");
  expect(rows).toHaveLength(batch.line_count + 1);

  // Sample receipts: every line recorded paid.
  const lines = page.getByRole("table", { name: "Lines in this batch" });
  for (const [i, l] of (batch.lines as Array<{ public_code: string }>).entries()) {
    await lines.getByRole("button", { name: `Record result for ${l.public_code}` }).click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("M-Pesa receipt").fill(`DEMO${run.toUpperCase().slice(-5)}${i}`.slice(0, 12));
    await dialog.getByRole("button", { name: "Record paid" }).click();
    await expect(lines.getByRole("row", { name: new RegExp(l.public_code) })).toContainText("DEMO");
  }
  await expect(page.getByText(/^Settled on /)).toBeVisible();
  await shot(page, "7-payout-settled");

  const r = batchReconciliation(batchId);
  expect(r.pending).toBe(0);
  expect(r.paid).toBe(batch.total_cents);
  expect(r.ledger, "the ledger holds a payout entry for every paid line").toBe(-r.paid);
});
