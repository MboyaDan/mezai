import AxeBuilder from "@axe-core/playwright";
import { expect, request, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";

// Stage 29 (D-171). Google sign-in for contributors. CI runs the API without
// a Google client, so the buttons must be absent there; locally, with
// GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET set, the spec checks the buttons
// and what they send, without leaving for Google. The error notices from
// ?error= work either way.

const base = `http://localhost:${Number(process.env.PORT ?? 3000)}`;
const shots = "../docs/screenshots/stage-29";

async function axe(page: Page, label: string) {
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  );
  const r = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
    .analyze();
  const blocking = r.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
  expect(blocking, `${label}: ${blocking.map((v) => `${v.id} (${v.nodes.length})`).join(", ")}`).toEqual([]);
}

async function shot(page: Page, file: string) {
  mkdirSync(shots, { recursive: true });
  await page.addStyleTag({ content: "nextjs-portal { display: none !important; }" });
  await page.screenshot({ path: `${shots}/${file}-1440.png`, fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.waitForTimeout(300);
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
    `${file} at 390`,
  ).toBeLessThanOrEqual(390);
  await page.screenshot({ path: `${shots}/${file}-390.png`, fullPage: true });
  await page.setViewportSize({ width: 1440, height: 900 });
}

test("29: Google sign-in buttons follow the API switch", async ({ page }) => {
  const api = await request.newContext({ baseURL: base });
  const on = (await (await api.get("/api/v1/auth/providers")).json()).google === true;
  const button = (p: Page) => p.getByRole("button", { name: "Continue with Google" });
  const link = (p: Page) => p.getByRole("link", { name: "Continue with Google" });
  // Alerts are also copied to a live region for screen readers: look in the page.
  const text = (p: Page, t: string) => p.locator("#main").getByText(t);

  // The page explains why the API sent the person back.
  await page.goto("/login?error=google_no_account");
  await expect(text(page, "No MezAI account uses that Google account yet.")).toBeVisible();
  await page.goto("/login?error=<b>not-a-code</b>");
  await expect(page.getByText("not-a-code")).toHaveCount(0);

  await page.goto("/login");
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  if (!on) {
    await expect(link(page)).toHaveCount(0);
    await page.goto("/join");
    await expect(page.getByRole("button", { name: "Create account" })).toBeVisible();
    await expect(button(page)).toHaveCount(0);
    return;
  }

  // Sign-in: a plain link to the API, which redirects to Google.
  await expect(link(page)).toHaveAttribute("href", "/api/v1/auth/google/start?intent=signin");
  await axe(page, "sign-in with Google");
  await shot(page, "login-google");

  // Sign-up: nothing leaves for Google until both documents are accepted.
  const started: string[] = [];
  await page.route("**/api/v1/auth/google/start**", (r) => {
    started.push(r.request().url());
    return r.fulfill({ status: 200, contentType: "text/plain", body: "stopped before Google" });
  });
  await page.goto("/join?error=google_cancelled");
  await expect(text(page, "Google sign-in was cancelled.")).toBeVisible();
  await button(page).click();
  await expect(text(page, "Read and accept the privacy notice to continue.")).toBeVisible();
  expect(started).toEqual([]);
  await axe(page, "join with Google, consents missing");
  await shot(page, "join-google");
  for (const box of await page.getByRole("checkbox").all()) await box.check();
  await button(page).click();
  await expect(page.getByText("stopped before Google")).toBeVisible();
  const url = new URL(started[0]);
  expect(url.searchParams.get("intent")).toBe("signup");
  expect(url.searchParams.get("privacy_notice")).toBe("0.5");
  expect(url.searchParams.get("contributor_terms")).toBeTruthy();
});
