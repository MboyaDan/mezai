import AxeBuilder from "@axe-core/playwright";
import { expect, request, test, type Page } from "@playwright/test";
import { mkdirSync } from "node:fs";
import { apiSignIn, signIn } from "./auth";

// Stage 25 (brief 6.3, D-167). Ops holds the quality figures and workforce
// back from the client on one of the seeded Sample Client projects; the Client
// Admin then sees progress without them, and the API refuses the hidden panel.
// The setting is put back at the end, so reruns and other specs see the seed.

const port = Number(process.env.PORT ?? 3000);
const base = `http://localhost:${port}`;
const shots = "../docs/screenshots/stage-25";

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

test("25: Ops chooses what the client sees on a project", async ({ page, browser }) => {
  test.setTimeout(180_000);
  const ops = await request.newContext({ baseURL: base });
  const h = await apiSignIn(ops, "superadmin@sample.invalid");
  const orgs = await (await ops.get("/api/v1/organisations")).json();
  const org = orgs.organisations.find((o: { slug: string }) => o.slug === "sample-client-ltd");
  const list = await (await ops.get(`/api/v1/projects?org_id=${org.id}`)).json();
  const project = (list.projects ?? list).find(
    (p: { item_count: number; status: string }) => p.item_count > 0 && p.status !== "draft",
  );
  expect(project, "a seeded Sample Client project with items").toBeTruthy();
  const reset = () =>
    ops.put(`/api/v1/projects/${project.id}/client-visibility`, {
      headers: h,
      data: { quality: true, workforce: true },
    });

  try {
    // Ops turns both switches off on the project page.
    await signIn(page, "superadmin@sample.invalid", /\/w/);
    await page.goto(`/w/projects/${project.id}`);
    const card = page.getByRole("region", { name: "What the client sees" });
    const quality = card.getByRole("switch", { name: "Quality figures" });
    const workforce = card.getByRole("switch", { name: "Workforce" });
    await expect(quality).toHaveAttribute("aria-checked", "true");
    await quality.click();
    await expect(quality).toHaveAttribute("aria-checked", "false");
    await expect(page.getByText("Quality hidden from the client")).toBeVisible();
    await workforce.click();
    await expect(workforce).toHaveAttribute("aria-checked", "false");
    await expect(page.getByText("Hidden from the client", { exact: true })).toBeVisible();
    // Staff still see every figure.
    await expect(page.getByText("Gold accuracy").first()).toBeVisible();
    await card.scrollIntoViewIfNeeded();
    await axe(page, "ops project page");
    await shot(page, "ops-project");

    // The Client Admin sees progress only.
    const client = await browser.newContext({ baseURL: base });
    const cp = await client.newPage();
    await signIn(cp, "client.admin@sample.invalid", /\/w/);
    await cp.goto(`/w/projects/${project.id}`);
    await expect(cp.getByRole("heading", { name: "Progress", exact: true })).toBeVisible();
    await expect(cp.getByText("Final items").first()).toBeVisible();
    await expect(cp.getByText("Gold accuracy")).toHaveCount(0);
    await expect(cp.getByRole("heading", { name: "Workforce" })).toHaveCount(0);
    await expect(cp.getByRole("region", { name: "What the client sees" })).toHaveCount(0);
    const refused = await cp.request.get(`/api/v1/projects/${project.id}/quality`);
    expect(refused.status()).toBe(403);
    expect((await refused.json()).error).toBe("not_shared");
    await axe(cp, "client project page");
    await shot(cp, "client-project");

    // Sharing again brings the figures back for the client.
    expect((await reset()).ok()).toBeTruthy();
    await cp.reload();
    await expect(cp.getByRole("heading", { name: "Progress and quality" })).toBeVisible();
    await expect(cp.getByRole("heading", { name: "Workforce" })).toBeVisible();
    await client.close();
  } finally {
    await reset();
  }
});
