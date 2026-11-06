import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";

// AC-04 through the real UI: sign up with consent, confirm the email, complete
// the profile, pass a qualification test, and be approved by Ops.

const mailpit = process.env.MAILPIT_URL ?? "http://localhost:8025";
const seedPassword = process.env.SEED_PASSWORD ?? "sample-password-only";
const shots = "../docs/screenshots/stage-07b";
const email = `e2e.${Date.now()}@sample.invalid`;
const password = "an end to end test passphrase";
const name = "Sample Joiner";

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
  await expect(page).toHaveURL(/\/login/);
}

test.describe.configure({ mode: "serial" });

test("AC-04: sign up, consent, profile, qualify, approval", async ({ page }) => {
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
  await page.getByRole("textbox", { name: "M-Pesa number", exact: true }).fill("0712 345 678");
  await page.getByLabel("Enter it again").fill("0712 345 679");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.getByText("The two numbers do not match.")).toBeVisible();
  await page.getByLabel("Enter it again").fill("0712345678");
  await axe(page, "/c/profile");
  await page.getByRole("button", { name: "Save profile" }).click();
  await expect(page.getByText("Profile complete")).toBeVisible();
  await expect(page.getByText("+254 7•• ••• 678")).toBeVisible();
  await shot(page, "profile");

  // 4. Take the classification test.
  await page.getByRole("link", { name: "Choose a qualification test" }).click();
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

  // 5. Ops reviews and approves.
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
  await expect(page.getByText("qualifying → ")).toBeVisible();
  await axe(page, "/w/contributors/[id]");
  await shot(page, "ops-contributor");
  await signOut(page);

  // 6. The contributor is active.
  await signIn(page, email, password);
  await expect(page.getByText("Active", { exact: true })).toBeVisible();
  await expect(page.getByRole("heading", { name: "You are active" })).toBeVisible();
  await shot(page, "dashboard-active");
});
