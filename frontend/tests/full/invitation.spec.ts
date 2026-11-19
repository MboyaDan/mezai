import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { signIn } from "./auth";

// Stage 19f: an invitation email's link opens /accept-invitation, which
// creates the account; the new person then signs in to their organisation.
// Found by the pilot demo run: the API accepted invitations but the site had
// no page for the link.

const mailpit = process.env.MAILPIT_URL ?? "http://localhost:8025";
const run = Date.now().toString(36);
const colleague = `invited.${run}@sample.invalid`;

async function invitationPath(to: string): Promise<string> {
  for (let i = 0; i < 40; i++) {
    const found = await (
      await fetch(`${mailpit}/api/v1/search?query=${encodeURIComponent(`to:${to}`)}`)
    ).json();
    const m = (found.messages ?? [])[0];
    if (m) {
      const msg = await (await fetch(`${mailpit}/api/v1/message/${m.ID}`)).json();
      const link = /https?:\/\/\S+\/accept-invitation\?token=[\w%-]+/.exec(msg.Text)?.[0];
      if (link) return new URL(link).pathname + new URL(link).search;
    }
    await new Promise((r) => setTimeout(r, 500));
  }
  throw new Error(`no invitation email for ${to}`);
}

test("19f: a client admin invites a colleague, who accepts and signs in", async ({ page }) => {
  await signIn(page, "client.admin@sample.invalid", /\/w$/);
  await page.getByRole("navigation").first().getByRole("link", { name: "Team" }).click();
  await page.getByLabel("Work email").fill(colleague);
  await page.getByRole("button", { name: "Send invitation" }).click();
  await expect(page.getByRole("row").filter({ hasText: colleague })).toBeVisible();
  await page.getByRole("button", { name: "Sign out" }).first().click();
  await expect(page).toHaveURL(/\/login/, { timeout: 20_000 });

  const path = await invitationPath(colleague);
  await page.goto("/accept-invitation");
  await expect(page.getByRole("heading", { name: "Invitation link incomplete" })).toBeVisible();

  await page.goto(path);
  await expect(page.getByRole("heading", { name: "Accept your invitation" })).toBeVisible();
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByText("Enter your name as colleagues should see it.")).toBeVisible();
  await page.getByLabel("Your name").filter({ visible: true }).fill("Sample Invited Colleague");
  await page.getByLabel("Password").filter({ visible: true }).fill("an invited colleague passphrase");
  const blocking = (
    await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
      .analyze()
  ).violations.filter((v) => v.impact === "critical" || v.impact === "serious");
  expect(blocking.map((v) => v.id)).toEqual([]);
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Your account is ready" })).toBeVisible();

  // The link works once.
  await page.goto(path);
  await page.getByLabel("Your name").filter({ visible: true }).fill("Someone Else");
  await page.getByLabel("Password").filter({ visible: true }).fill("another long passphrase");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByText("This link is invalid or has expired.")).toBeVisible();

  await signIn(page, colleague, /\/w$/, "an invited colleague passphrase");
  // A client user lands on their organisation's workspace.
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await expect(page.getByText("Sample Invited Colleague").first()).toBeVisible();
});

test("19f: Ops invites a client admin into an organisation, and a staff member", async ({ page }) => {
  const clientAdmin = `org.admin.${run}@sample.invalid`;
  const staff = `new.reviewer.${run}@sample.invalid`;
  await signIn(page, "ops@sample.invalid", /\/w$/);

  await page.goto("/w/organisations");
  const panel = page.getByRole("region", { name: "Invite a client user" });
  await panel.getByLabel("Work email").fill(clientAdmin);
  await panel.getByRole("button", { name: "Send invitation" }).click();
  await expect(panel.getByText("Choose the organisation.")).toBeVisible();
  await panel.getByLabel("Organisation").selectOption({ label: "Other Sample Org (Sample)" });
  await panel.getByLabel("Role").selectOption("client_admin");
  await panel.getByRole("button", { name: "Send invitation" }).click();
  await expect(panel.getByRole("row").filter({ hasText: clientAdmin })).toContainText(
    "Other Sample Org (Sample)",
  );

  await page.goto("/w/staff");
  const staffPanel = page.getByRole("region", { name: "Invite a staff member" });
  await staffPanel.getByLabel("Work email").fill(staff);
  await staffPanel.getByLabel("Role").selectOption("reviewer");
  await staffPanel.getByRole("button", { name: "Send invitation" }).click();
  const row = staffPanel.getByRole("row").filter({ hasText: staff });
  await expect(row).toContainText("Reviewer");
  await row.getByRole("button", { name: `Cancel the invitation for ${staff}` }).click();
  await expect(row).toHaveCount(0);
  await page.getByRole("button", { name: "Sign out" }).first().click();
  await expect(page).toHaveURL(/\/login/, { timeout: 20_000 });

  await page.goto(await invitationPath(clientAdmin));
  await page.getByLabel("Your name").filter({ visible: true }).fill("Sample Org Admin");
  await page.getByLabel("Password").filter({ visible: true }).fill("an organisation admin passphrase");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Your account is ready" })).toBeVisible();
  await signIn(page, clientAdmin, /\/w$/, "an organisation admin passphrase");
  await expect(page.getByText("Sample Org Admin").first()).toBeVisible();
});
