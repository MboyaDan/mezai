import { expect, type APIRequestContext, type Page } from "@playwright/test";

// Sign-in for the full-stack specs. The API allows 20 sign-ins a minute per
// IP address, and in CI every spec signs in from the same address. The suite
// respects that limit instead of raising it: a refused sign-in waits for the
// window to reset (at most a minute) and tries once more.

export const seedPassword = process.env.SEED_PASSWORD ?? "sample-password-only";
const port = Number(process.env.PORT ?? 3000);
const origin = `http://localhost:${port}`;

/** Signs in through the login form and waits for the account's home page. */
export async function signIn(page: Page, who: string, home: RegExp, password = seedPassword) {
  for (let attempt = 0; ; attempt++) {
    await page.goto("/login");
    await page.getByLabel("Email").filter({ visible: true }).fill(who);
    await page.getByLabel("Password").filter({ visible: true }).fill(password);
    const answer = page.waitForResponse((r) => r.url().endsWith("/api/v1/auth/login"));
    await page.getByRole("button", { name: "Sign in" }).click();
    const res = await answer;
    if (res.status() === 429 && attempt === 0) {
      await page.waitForTimeout(retryAfterMs(res.headers()["retry-after"]));
      continue;
    }
    await expect(page).toHaveURL(home);
    return;
  }
}

/** Signs in through the API and returns the CSRF headers for later calls. */
export async function apiSignIn(api: APIRequestContext, who: string, password = seedPassword) {
  for (let attempt = 0; ; attempt++) {
    const res = await api.post("/api/v1/auth/login", {
      headers: { Origin: origin },
      data: { email: who, password },
    });
    if (res.status() === 429 && attempt === 0) {
      await new Promise((r) => setTimeout(r, retryAfterMs(res.headers()["retry-after"])));
      continue;
    }
    expect(res.ok(), await res.text()).toBeTruthy();
    const { csrf_token: csrf } = await res.json();
    return { "X-CSRF-Token": csrf as string, Origin: origin };
  }
}

function retryAfterMs(header: string | undefined) {
  const s = Number(header);
  return (Number.isFinite(s) && s > 0 ? Math.min(s, 65) : 61) * 1000;
}
