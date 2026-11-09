import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";

// AC-04 through the real UI: sign up with consent, confirm the email, complete
// the profile, pass a qualification test, be approved by Ops and have the
// identity verified (D-078).

const mailpit = process.env.MAILPIT_URL ?? "http://localhost:8025";
const seedPassword = process.env.SEED_PASSWORD ?? "sample-password-only";
const shots = "../docs/screenshots/stage-07d1";
const stamp = String(Date.now()).slice(-8);
const email = `e2e.${stamp}@sample.invalid`;
// One account per number (D-078), so each run uses fresh numbers.
const phone = `07${stamp}`;
const nationalId = `3${stamp.slice(1)}`;
const password = "an end to end test passphrase";
const name = `Sample Joiner ${stamp.slice(-4)}`;

// Seed sample text → correct label (backend/seed/content.go).
const routing: Record<string, string> = {
  "Nimeongeza salio lakini bado halijaonekana.": "Billing",
  "Mtandao uko chini sana leo asubuhi.": "Network",
  "Nimesahau nenosiri la akaunti yangu.": "Account",
  "Asante kwa huduma nzuri jana.": "Other",
};

async function axe(page: Page, label: string) {
  const r = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  const blocking = r.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
  expect(blocking, `${label}: ${blocking.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`).toEqual([]);
}

// Screenshots for the PR at 1440 and 390, without the Next.js dev badge or
// toasts, from the top of the page. At 390 the page must not scroll sideways.
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

async function verificationLink(to: string): Promise<string> {
  for (let i = 0; i < 30; i++) {
    const list = await (await fetch(`${mailpit}/api/v1/messages?limit=50`)).json();
    const m = list.messages?.find((x: { To: Array<{ Address: string }> }) =>
      x.To.some((t) => t.Address === to),
    );
    if (m) {
      const msg = await (await fetch(`${mailpit}/api/v1/message/${m.ID}`)).json();
      const link = /https?:\/\/\S+\/verify-email\?token=[\w%-]+/.exec(msg.Text)?.[0];
      if (link) return new URL(link).pathname + new URL(link).search;
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`no verification email for ${to}`);
}

async function signIn(page: Page, who: string, pw: string) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(who);
  await page.getByLabel("Password").fill(pw);
  await page.getByRole("button", { name: "Sign in" }).click();
}

async function signOut(page: Page) {
  const menu = page.getByRole("button", { name: /Account menu/ });
  if (await menu.isVisible()) {
    await menu.click();
    await page.getByRole("button", { name: "Sign out" }).click();
  } else {
    await page.getByRole("button", { name: "Sign out" }).first().click();
  }
  // Full page load of /login; the dev server may compile it first.
  await expect(page).toHaveURL(/\/login/, { timeout: 20_000 });
}

test.describe.configure({ mode: "serial" });

test("AC-04: sign up, consent, profile, qualify, approval", async ({ page }) => {
  // The longest flow in the suite: 12 screenshots at two widths and axe checks.
  test.setTimeout(180_000);
  // Signed-out visitors to the portal are sent to sign in.
  await page.goto("/c/profile");
  await expect(page).toHaveURL(/\/login\?next=%2Fc%2Fprofile/);

  // 1. Sign up. Consent is required.
  await page.goto("/join");
  await axe(page, "/join");
  await shot(page, "join");
  await page.getByLabel("Display name").fill(name);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByText(/accept the privacy notice to continue/i)).toBeVisible();
  await page.getByLabel(/accept the privacy notice/i).check();
  await page.getByLabel(/accept the contributor terms/i).check();
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Check your email" })).toBeVisible();

  // Signing in before confirming is refused with a way to resend.
  await signIn(page, email, password);
  await expect(page.getByText("Confirm your email address first")).toBeVisible();
  await axe(page, "/login (unverified)");

  // 2. Confirm the email.
  await page.goto(await verificationLink(email));
  await expect(page.getByRole("heading", { name: "Email confirmed" })).toBeVisible();

  // 3. Sign in and complete the profile.
  await signIn(page, email, password);
  await expect(page).toHaveURL(/\/c$/);
  await expect(page.getByRole("heading", { name: "Welcome, Sample" })).toBeVisible();
  await expect(page.getByText("Applied")).toBeVisible();
  await axe(page, "/c");
  await shot(page, "dashboard-new");

  await page.getByRole("link", { name: "Complete profile" }).click();
  await expect(page.getByRole("heading", { name: "Your profile" })).toBeVisible();
  await page.getByLabel("Highest education").selectOption("bachelor");
  await page.getByLabel("Hours a week you can work").fill("15");
  await page.getByRole("combobox", { name: "Language 1" }).selectOption("sw");
  await page.getByRole("combobox", { name: "Level" }).selectOption("native");
  await page.getByLabel("Dialects").fill("Sheng");
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
  await page.getByRole("textbox", { name: "M-Pesa number", exact: true }).fill(phone);
  await page.getByLabel("Enter it again").fill("0700000000");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.getByText("The two numbers do not match.")).toBeVisible();
  await page.getByLabel("Enter it again").fill(phone);
  await page.getByRole("textbox", { name: "National ID number" }).fill("12AB");
  await axe(page, "/c/profile");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.getByText("Enter your National ID number: 6 to 9 digits.")).toBeVisible();
  await page.getByRole("textbox", { name: "National ID number" }).fill(nationalId);
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.getByText("Application complete")).toBeVisible();
  await expect(page.getByText(`+254 7•• ••• ${phone.slice(-3)}`)).toBeVisible();
  await expect(page.getByText(`ID ending ${nationalId.slice(-3)}`)).toBeVisible();
  await shot(page, "profile");

  // 4. Ops screens the application and shortlists it (D-100); tests open.
  await page.goto("/c/qualify");
  await expect(page.getByText("Tests open after we review your application")).toBeVisible();
  await signOut(page);
  await signIn(page, "ops@sample.invalid", seedPassword);
  await expect(page).toHaveURL(/\/w$/);
  await page.goto("/w/contributors?tab=applications");
  await page
    .getByRole("row")
    .filter({ hasText: name })
    .getByRole("link", { name: /Review/ })
    .click();
  await expect(page.getByRole("heading", { name: "Application", exact: true })).toBeVisible();
  await expect(page.getByText("Sample diploma in journalism")).toBeVisible();
  await shot(page, "ops-application");
  await page.getByRole("button", { name: "Shortlist", exact: true }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Shortlist" }).click();
  await expect(page.getByRole("button", { name: "Shortlist", exact: true })).toHaveCount(0);
  await signOut(page);
  await signIn(page, email, password);
  await expect(page).toHaveURL(/\/c$/);

  // 5. Take the classification test.
  await page.goto("/c/qualify");
  await expect(page.getByRole("heading", { name: "Qualification", exact: true })).toBeVisible();
  await axe(page, "/c/qualify");
  await shot(page, "qualify");
  const card = page.getByRole("listitem").filter({ hasText: "Swahili message routing test (Sample)" });
  await card.getByRole("link", { name: "Start" }).click();
  await page.getByRole("button", { name: "Begin the test" }).click();
  for (let i = 1; i <= 4; i++) {
    await expect(page.getByRole("heading", { name: `Question ${i} of 4` })).toBeVisible();
    const text = (await page.locator("section [class*=whitespace-pre-wrap]").first().innerText()).trim();
    const label = routing[text];
    expect(label, `unknown sample text: ${text}`).toBeTruthy();
    if (i === 2) {
      // Keyboard: number keys pick labels (Network is the second label).
      await page.keyboard.press("2");
      await expect(page.getByRole("radio", { name: /Network/ })).toBeChecked();
    } else {
      await page.getByRole("radio", { name: new RegExp(label) }).check();
    }
    if (i === 1) {
      await axe(page, "test question");
      await shot(page, "test-question");
    }
    if (i < 4) await page.getByRole("button", { name: "Next", exact: true }).click();
  }
  await expect(page.getByText("All answers saved")).toBeVisible();
  await page.getByRole("button", { name: "Review and submit" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Submit" }).click();
  await expect(page.getByRole("heading", { name: "Submitted" })).toBeVisible();
  await expect(page.getByText("100")).toBeVisible();
  await shot(page, "test-result");
  await signOut(page);

  // 6. Ops reviews and approves.
  await signIn(page, "ops@sample.invalid", seedPassword);
  await expect(page).toHaveURL(/\/w$/);
  await page.getByRole("link", { name: "Contributors" }).first().click();
  await expect(page.getByRole("tab", { name: "Waiting for review" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await axe(page, "/w/contributors");
  await shot(page, "ops-queue");
  const row = page.getByRole("row").filter({ hasText: name });
  await row.getByRole("link", { name: /Review/ }).click();
  await expect(page.getByRole("heading", { name: "Swahili message routing test (Sample)" })).toBeVisible();
  await expect(page.getByText("Matches")).toHaveCount(4);
  await axe(page, "attempt review");
  await shot(page, "ops-review");
  await page.getByRole("button", { name: "Approve" }).click();
  await expect(page.getByText("Approved", { exact: true })).toBeVisible();
  await page.getByRole("link", { name: /C-\d+ Sample Joiner/ }).click();
  // Approved but still qualifying: the identity check is outstanding (D-078).
  await expect(page.getByText("applied → ")).toBeVisible();
  await expect(page.getByText("qualifying → ")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "Identity", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Verify identity" })).toBeEnabled();
  await axe(page, "/w/contributors/[id]");
  await shot(page, "ops-contributor");

  // 7. Ops verifies the identity (D-078).
  await page.getByRole("button", { name: "Verify identity" }).click();
  const dialog = page.getByRole("dialog");
  // A note is optional without possible duplicates; it goes to the audit log.
  await dialog.getByLabel(/Note/).fill("Numbers checked; no other account found.");
  await axe(page, "verify identity dialog");
  await dialog.getByRole("button", { name: "Verify", exact: true }).click();
  // Verified: the panel offers only Reject now, and the history shows activation.
  await expect(page.getByRole("button", { name: "Verify identity" })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Reject identity" })).toBeVisible();
  await shot(page, "ops-identity-verified");

  // 8. The structured phone interview completes activation (D-100).
  await expect(page.getByText("qualifying → ")).toHaveCount(0);
  await page.getByRole("button", { name: "Record interview" }).click();
  const iv = page.getByRole("dialog", { name: /Interview: C-/ });
  await iv.getByLabel("Name and ID digits match").check();
  await iv.getByRole("group", { name: "Kiswahili" }).getByLabel("4 out of 4").check();
  await iv.getByRole("group", { name: "Scenario score" }).getByLabel("3 out of 4").check();
  await iv.getByLabel("They keep task content private").check();
  await iv.getByLabel("Hours, device and connection").fill("15 hours a week, smartphone, good signal");
  await expect(iv.getByText("Result if saved: passed.")).toBeVisible();
  await axe(page, "interview drawer");
  await shot(page, "ops-interview");
  await iv.getByRole("button", { name: "Save interview" }).click();
  await expect(page.getByText(/Interview passed · /)).toBeVisible();
  await expect(page.getByText("qualifying → ")).toBeVisible();
  await page.goto("/w/contributors?tab=identity");
  await expect(page.getByRole("tab", { name: "Identity checks" })).toHaveAttribute("aria-selected", "true");
  await axe(page, "identity queue");
  await shot(page, "ops-identity-queue");
  await signOut(page);

  // 9. The contributor is active.
  await signIn(page, email, password);
  await expect(page.getByText("Active", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "You are active" })).toBeVisible();
  await shot(page, "dashboard-active");
});
