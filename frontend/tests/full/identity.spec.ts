import AxeBuilder from "@axe-core/playwright";
import { expect, request, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { apiSignIn, signIn } from "./auth";

// Stage 17a (D-136 to D-138): one person, one account, on screen. An
// account that keeps entering other people's numbers stops getting answers;
// Ops sees that number changes are paused; the contributor sees plainly what
// is kept; and the signed-in pages trip nothing in the content security
// policy. Safe to run again: tries made by an earlier run still count.

const shots = "../docs/screenshots/stage-17a";
const applicant = "contributor010@sample.invalid";
const base = `http://localhost:${Number(process.env.PORT ?? 3000)}`;

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

test("number tries are capped, Ops sees the pause, and the pages respect the policy", async ({ page }) => {
  const violations: string[] = [];
  page.on("console", (m) => {
    if (/Content Security Policy/i.test(m.text())) violations.push(m.text());
  });

  // The applicant enters the M-Pesa numbers of four other sample accounts.
  const api = await request.newContext({ baseURL: base });
  const headers = await apiSignIn(api, applicant);
  const answers: string[] = [];
  for (const n of ["01", "02", "03", "04"]) {
    const res = await api.put("/api/v1/me/contributor/profile", {
      headers,
      data: {
        display_name: "Sample Contributor 010",
        country: "ke",
        city: "Nairobi",
        education_level: "bachelor",
        weekly_hours: 15,
        domains: ["health"],
        languages: [{ language: "sw", proficiency: "native" }],
        mpesa_phone: `07000000${n}`,
        mpesa_phone_confirm: `07000000${n}`,
        national_id: "",
      },
    });
    expect(res.status()).toBe(422);
    answers.push((await res.json()).message as string);
  }
  // At most three answers say the number is taken; after that, none do.
  expect(answers.filter((a) => a.includes("another MezAI account")).length).toBeLessThanOrEqual(3);
  expect(answers[3]).toContain("cannot be changed for now");
  expect(answers[3]).not.toContain("another MezAI account");
  await api.dispose();

  // The contributor sees what is kept and what is not.
  await signIn(page, applicant, /\/c/);
  await page.goto("/c/profile");
  // A sample applicant may still have to accept the current documents.
  const consent = page.getByRole("button", { name: "Accept and continue" });
  await page.getByRole("heading", { level: 1 }).first().waitFor();
  if (await consent.isVisible()) {
    for (const box of await page.getByRole("checkbox", { name: /I have read and accept/ }).all())
      await box.check();
    await consent.click();
    await page.goto("/c/profile");
  }
  await expect(page.getByText("What we keep", { exact: true })).toBeVisible();
  await expect(page.getByText("What we never keep", { exact: true })).toBeVisible();
  await page.getByText("What we keep", { exact: true }).scrollIntoViewIfNeeded();
  await axe(page, "contributor profile");
  await shot(page, "profile-national-id");

  // Ops sees that number changes are paused on that account.
  await page.context().clearCookies();
  await signIn(page, "superadmin@sample.invalid", /\/w/);
  const list = await page.request.get("/api/v1/contributors");
  expect(list.ok()).toBeTruthy();
  const rows = (await list.json()).contributors as Array<{ id: string; public_code: string }>;
  const target = rows.find((r) => r.public_code === "C-0010");
  expect(target, "sample contributor C-0010").toBeTruthy();
  await page.goto(`/w/contributors/${target!.id}`);
  await expect(page.getByText("Number changes are paused on this account")).toBeVisible();
  await axe(page, "ops identity panel");
  await shot(page, "ops-identity-paused");

  expect(violations).toEqual([]);
});
