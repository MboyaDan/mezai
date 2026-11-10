import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { signIn as signInAs } from "./auth";

const signIn = (page: Page, who: string) => signInAs(page, who, /\/w$/);

// AC-05 through the real UI: a client admin requests a project, Ops converts
// it, imports items (a file with row errors imports nothing, a valid one
// imports every row), adds gold, makes a qualification test and publishes.
// The client then sees its own project without staff-only settings, and a
// client of another organisation cannot open it.

const shots = "../docs/screenshots/stage-08b";
const run = Date.now().toString(36);
const title = `Support ticket routing ${run} (Sample)`;

async function axe(page: Page, label: string) {
  const r = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  const blocking = r.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
  expect(blocking, `${label}: ${blocking.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`).toEqual([]);
}

// 1440 and 390 screenshots; at 390 the page must not scroll sideways.
async function shot(page: Page, file: string) {
  mkdirSync(shots, { recursive: true });
  await page.addStyleTag({
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
}

const tabs = (page: Page) => page.getByRole("navigation", { name: "Project" });

async function signOut(page: Page) {
  await page.getByRole("button", { name: "Sign out" }).first().click();
  await expect(page).toHaveURL(/\/login/);
}

// Harmless everyday sentences written for the sample (CLAUDE.md seed rules).
const goodCsv = [
  "external_id,text",
  `${run}-1,Nimelipa bili lakini huduma bado haijarudi.`,
  `${run}-2,Simu yangu haipati mtandao tangu jana.`,
  `${run}-3,Ninataka kubadilisha nenosiri langu.`,
].join("\n");
// Row 3 has no text: the whole file must import nothing.
const badCsv = ["external_id,text", `${run}-x1,Habari ya asubuhi.`, `${run}-x2,`].join("\n");

const file = (name: string, body: string) => ({ name, mimeType: "text/csv", buffer: Buffer.from(body) });

test.describe.configure({ mode: "serial" });

let requestUrl = "";
let projectUrl = "";

test("AC-05a: a client admin requests a project", async ({ page }) => {
  await signIn(page, "client.admin@sample.invalid");
  await page.getByRole("link", { name: "Requests" }).first().click();
  await expect(page.getByRole("heading", { level: 1, name: "Project requests" })).toBeVisible();
  await page.getByRole("link", { name: "Request a project" }).click();

  // Client-side checks name every missing field before anything is sent.
  await page.getByRole("button", { name: "Send request" }).click();
  await expect(page.getByText("Give the request a short title.").filter({ visible: true })).toBeVisible();

  await page.getByLabel("Title").filter({ visible: true }).fill(title);
  await page.getByLabel("Text classification").filter({ visible: true }).check();
  await page
    .getByLabel(/^Brief/)
    .filter({ visible: true })
    .fill("Route short customer messages in Kiswahili to the right team. Sample request for testing.");
  await page.getByLabel("Kiswahili").filter({ visible: true }).check();
  await page.getByLabel("Rough number of items").filter({ visible: true }).fill("3");
  await page.getByLabel("Sample file").filter({ visible: true }).setInputFiles(file("sample.csv", goodCsv));
  await axe(page, "/w/requests/new");
  await shot(page, "request-new");
  await page.getByRole("button", { name: "Send request" }).click();

  await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
  await expect(page.getByText("Waiting for MezAI").filter({ visible: true })).toBeVisible();
  await expect(page.getByRole("link", { name: /Download sample: sample\.csv/ })).toBeVisible();
  // Clients cannot convert their own request.
  await expect(page.getByRole("button", { name: "Create project from this" })).toHaveCount(0);
  requestUrl = page.url();
  await signOut(page);
});

test("AC-05b: Ops converts the request, imports items and gold, and publishes", async ({ page }) => {
  await signIn(page, "ops@sample.invalid");
  await page.goto(requestUrl);
  await axe(page, "/w/requests/[id]");
  await page.getByRole("button", { name: "Create project from this" }).click();

  // The request's languages became a required skill; labels are still needed.
  await page.getByRole("button", { name: "Create draft project" }).click();
  await expect(
    page.getByText("List at least two labels, one per line.").filter({ visible: true }),
  ).toBeVisible();
  await page
    .getByLabel(/^Labels/)
    .filter({ visible: true })
    .fill("Billing\nNetwork\nAccount\nOther");
  await page
    .getByLabel(/^Pay per approved task/)
    .filter({ visible: true })
    .fill("12.50");
  await page
    .getByLabel(/^Contributor instructions/)
    .filter({ visible: true })
    .fill("Read the message and choose the one team that should handle it.");
  await page.getByRole("button", { name: "Create draft project" }).click();

  await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
  projectUrl = page.url();
  await expect(page.getByText("Draft", { exact: true }).filter({ visible: true })).toBeVisible();
  await expect(page.getByText("KES 12.50").filter({ visible: true })).toBeVisible();
  await axe(page, "/w/projects/[id]");

  // Publishing is offered only after "Mark ready", and needs items and gold.
  await page.getByRole("button", { name: "Mark ready" }).click();
  await expect(page.getByText("Ready", { exact: true }).filter({ visible: true })).toBeVisible();
  await page.getByRole("button", { name: "Publish" }).click();
  await expect(page.getByText("Import at least one item").filter({ visible: true })).toBeVisible();
  await expect(
    page.getByText("Add gold items, or set the gold rate to 0").filter({ visible: true }),
  ).toBeVisible();

  // Items: a file with a bad row imports nothing.
  await tabs(page).getByRole("link", { name: "Items", exact: true }).click();
  await page.getByLabel(/^File/).filter({ visible: true }).setInputFiles(file("bad.csv", badCsv));
  await page.getByRole("button", { name: "Check file" }).click();
  await expect(
    page.getByText("bad.csv: fix the problems below and upload again").filter({ visible: true }),
  ).toBeVisible();
  await expect(page.getByRole("table", { name: /Problems in bad\.csv/ })).toContainText("3");
  await expect(page.getByRole("button", { name: /^Import \d+ rows/ })).toHaveCount(0);
  await axe(page, "/w/projects/[id]/items (report)");
  await shot(page, "items-report");
  await page.getByRole("button", { name: "Upload another file" }).click();

  // A valid file is checked, then imported in full.
  await page.getByLabel(/^File/).filter({ visible: true }).setInputFiles(file("good.csv", goodCsv));
  await page.getByRole("button", { name: "Check file" }).click();
  await expect(page.getByText("good.csv: every row is valid").filter({ visible: true })).toBeVisible();
  await page.getByRole("button", { name: "Import 3 rows" }).click();
  await expect(page.getByRole("table", { name: /Uploaded files/ })).toContainText("Imported");
  await expect(page.getByRole("table", { name: /Uploaded files/ })).toContainText("Has errors");
  await shot(page, "items");

  // Gold: two by hand, then a qualification test from one of them.
  await tabs(page).getByRole("link", { name: "Gold", exact: true }).click();
  for (const [text, label] of [
    ["Salio langu limeisha haraka sana.", "Billing"],
    ["Siwezi kuingia kwenye akaunti yangu.", "Account"],
  ]) {
    await page.getByRole("button", { name: "Add one by hand" }).click();
    const dialog = page.getByRole("dialog", { name: "Add a gold item" });
    await dialog.getByLabel("text").fill(text);
    await dialog.getByLabel(label, { exact: true }).check();
    await dialog.getByRole("button", { name: "Add gold item" }).click();
    await expect(page.getByRole("table", { name: /Gold items/ })).toContainText(text);
  }
  await expect(page.getByRole("heading", { name: "Gold set (2 active)" })).toBeVisible();
  await axe(page, "/w/projects/[id]/gold");
  await shot(page, "gold");

  await page.getByRole("button", { name: "Make a qualification test" }).click();
  const make = page.getByRole("dialog", { name: "Make a qualification test" });
  await make.getByLabel(/^Questions/).fill("1");
  await make.getByRole("button", { name: "Make draft test" }).click();
  await expect(
    make.getByText("Confirm the content may be shown to applicants.").filter({ visible: true }),
  ).toBeVisible();
  await make.getByLabel(/may be shown to applicants/).check();
  await make.getByRole("button", { name: "Make draft test" }).click();
  await expect(page.getByRole("heading", { name: "Qualification tests from this project" })).toBeVisible();
  // The copied gold item is retired here (D-063); one stays active.
  await expect(page.getByRole("heading", { name: "Gold set (1 active)" })).toBeVisible();

  // Now it can go live.
  await tabs(page).getByRole("link", { name: "Overview", exact: true }).click();
  await page.getByRole("button", { name: "Publish" }).click();
  await expect(page.getByText("Live", { exact: true }).filter({ visible: true })).toBeVisible();

  // Settings: labels and overlap are locked once there is gold.
  await tabs(page).getByRole("link", { name: "Settings", exact: true }).click();
  await expect(page.getByLabel(/^Labels/).filter({ visible: true })).toBeDisabled();
  await page.getByLabel("Target items").filter({ visible: true }).fill("300");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByLabel("Target items").filter({ visible: true })).toHaveValue("300");
  await axe(page, "/w/projects/[id]/settings");
  await shot(page, "settings");

  await page.goto("/w/projects");
  await expect(page.getByRole("link", { name: title })).toBeVisible();
  await axe(page, "/w/projects");
  await shot(page, "projects");
  await signOut(page);
});

test("AC-05c: the client sees its project without staff settings; another client cannot", async ({
  page,
}) => {
  await signIn(page, "client.admin@sample.invalid");
  await page.goto("/w/projects");
  await page.getByRole("link", { name: title }).click();
  await expect(page.getByRole("heading", { level: 1, name: title })).toBeVisible();
  await expect(page.getByText("Live", { exact: true }).filter({ visible: true })).toBeVisible();
  // Pay, gold counts and the staff tabs are not shown to clients.
  await expect(page.getByText("KES 12.50").filter({ visible: true })).toHaveCount(0);
  await expect(page.getByText("Active gold items").filter({ visible: true })).toHaveCount(0);
  await expect(tabs(page).getByRole("link", { name: "Gold", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Pause" })).toHaveCount(0);
  await axe(page, "/w/projects/[id] (client)");
  await shot(page, "project-client");
  await page.goto(requestUrl);
  await expect(
    page.getByText("A project was created from this request").filter({ visible: true }),
  ).toBeVisible();
  await signOut(page);

  await signIn(page, "other.admin@sample.invalid");
  await page.goto(projectUrl);
  await expect(page.getByText("We could not find that.").filter({ visible: true })).toBeVisible();
  await page.goto("/w/projects");
  await expect(page.getByRole("link", { name: title })).toHaveCount(0);
  await signOut(page);
});
