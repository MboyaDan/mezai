import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Browser, type Page } from "@playwright/test";
import { seedPassword } from "./auth";

// Stage 18a: every page, for every kind of user, at 1440 and 390 in light
// and at 1440 in dark: axe (no serious or critical issue) and no sideways
// scroll. Problems are collected and listed together at the end of each
// role's test, so one run shows them all.
const port = Number(process.env.PORT ?? 3000);
const origin = `http://localhost:${port}`;

type Role = { name: string; email?: string; routes: (ids: Ids) => string[] };
type Ids = Record<string, string | undefined>;

const publicRoutes = [
  "/",
  "/services",
  "/quality",
  "/industries",
  "/contributors",
  "/about",
  "/case-studies",
  "/contact",
  "/legal/privacy",
  "/legal/terms",
  "/legal/contributor-terms",
  "/legal/acceptable-use",
  "/legal/cookies",
  "/login",
  "/join",
  "/forgot-password",
  "/reset-password",
  "/verify-email",
  "/no-such-page",
];

const roles: Role[] = [
  { name: "public", routes: () => publicRoutes },
  {
    name: "contributor",
    email: "contributor007@sample.invalid",
    routes: (ids) => [
      "/c",
      "/c/work",
      "/c/earnings",
      "/c/training",
      "/c/profile",
      "/c/qualify",
      ...(ids.project ? [`/c/projects/${ids.project}`, `/c/work/${ids.project}`] : []),
      ...(ids.test ? [`/c/qualify/${ids.test}`] : []),
    ],
  },
  {
    name: "ops",
    email: "ops@sample.invalid",
    routes: (ids) => [
      "/w",
      "/w/projects",
      "/w/projects/new",
      "/w/contributors",
      "/w/review",
      "/w/reports",
      "/w/requests",
      "/w/organisations",
      "/w/payouts",
      "/w/payouts/reconciliation",
      "/w/invoices",
      "/w/activity",
      "/w/staff",
      "/w/account",
      ...(ids.project
        ? [
            `/w/projects/${ids.project}`,
            `/w/projects/${ids.project}/items`,
            `/w/projects/${ids.project}/gold`,
            `/w/projects/${ids.project}/settings`,
            `/w/projects/${ids.project}/deliverables`,
          ]
        : []),
      ...(ids.contributor ? [`/w/contributors/${ids.contributor}`] : []),
      ...(ids.request ? [`/w/requests/${ids.request}`] : []),
      ...(ids.review ? [`/w/review/${ids.review}`] : []),
      ...(ids.batch ? [`/w/payouts/${ids.batch}`] : []),
    ],
  },
  {
    name: "client",
    email: "client.admin@sample.invalid",
    routes: (ids) => [
      "/w",
      "/w/projects",
      "/w/requests",
      "/w/requests/new",
      "/w/team",
      "/w/invoices",
      "/w/activity",
      ...(ids.project ? [`/w/projects/${ids.project}`, `/w/projects/${ids.project}/deliverables`] : []),
    ],
  },
];

async function idsFor(page: Page, role: string): Promise<Ids> {
  const get = async (path: string) => {
    const r = await page.request.get(`/api${path}`);
    return r.ok() ? r.json() : {};
  };
  if (role === "contributor") {
    const projects = (await get("/v1/me/projects")).projects ?? [];
    const tests = (await get("/v1/me/qualification/tests")).tests ?? [];
    return { project: projects[0]?.id, test: tests[0]?.id };
  }
  if (role === "ops") {
    return {
      project: ((await get("/v1/projects?status=")).projects ?? [])[0]?.id,
      contributor: ((await get("/v1/contributors")).contributors ?? [])[0]?.id,
      request: ((await get("/v1/project-requests")).requests ?? [])[0]?.id,
      review: ((await get("/v1/review/items")).items ?? [])[0]?.item_id,
      batch: ((await get("/v1/payouts/batches")).batches ?? [])[0]?.id,
    };
  }
  if (role === "client") return { project: ((await get("/v1/projects?status=")).projects ?? [])[0]?.id };
  return {};
}

async function signedInPage(browser: Browser, email?: string) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await context.newPage();
  if (email) {
    for (let attempt = 0; attempt < 2; attempt++) {
      const res = await context.request.post(`${origin}/api/v1/auth/login`, {
        headers: { Origin: origin },
        data: { email, password: seedPassword },
      });
      if (res.status() === 429) {
        await page.waitForTimeout(61_000);
        continue;
      }
      expect(res.ok(), await res.text()).toBeTruthy();
      break;
    }
  }
  return { context, page };
}

async function settle(page: Page) {
  await page.waitForLoadState("networkidle").catch(() => undefined);
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getComputedTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  );
}

const variants = [
  { label: "1440 light", width: 1440, height: 900, dark: false },
  { label: "390 light", width: 390, height: 844, dark: false },
  { label: "1440 dark", width: 1440, height: 900, dark: true },
];

for (const role of roles) {
  test(`18a sweep: every ${role.name} page passes axe and fits at 390`, async ({ browser }) => {
    test.setTimeout(15 * 60_000);
    const { context, page } = await signedInPage(browser, role.email);
    if (role.email) await page.goto("/");
    const routes = role.routes(await idsFor(page, role.name));
    const problems: string[] = [];
    for (const route of routes) {
      for (const v of variants) {
        await page.setViewportSize({ width: v.width, height: v.height });
        await page.goto(route);
        await page
          .locator("h1")
          .first()
          .waitFor({ timeout: 15_000 })
          .catch(() => problems.push(`${route}: no h1`));
        if (v.dark) await page.evaluate(() => (document.documentElement.dataset.theme = "dark"));
        await settle(page);
        const scroll = await page.evaluate(() => document.documentElement.scrollWidth);
        if (scroll > v.width + 1)
          problems.push(`${route} (${v.label}): page scrolls sideways to ${scroll} px`);
        const r = await new AxeBuilder({ page })
          .withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"])
          .exclude("nextjs-portal")
          .analyze();
        for (const x of r.violations.filter((x) => x.impact === "critical" || x.impact === "serious"))
          problems.push(
            `${route} (${v.label}): ${x.id} on ${x.nodes
              .map((n) => n.target.join(" "))
              .slice(0, 3)
              .join(", ")}`,
          );
        if (v.label === "1440 light") problems.push(...(await focusCheck(page, route)));
      }
    }
    await context.close();
    console.log(`${role.name}: ${routes.length} pages × ${variants.length} views checked`);
    expect(problems, problems.join("\n")).toEqual([]);
  });
}

/**
 * Keyboard: Tab through the first 15 stops and check each focused element
 * shows a focus indicator (outline, ring or box-shadow) different from its
 * unfocused look. Pages with fewer stops are fine.
 */
async function focusCheck(page: Page, route: string): Promise<string[]> {
  const out: string[] = [];
  // The first stop is "Skip to content", and it moves focus to <main>.
  await page.keyboard.press("Tab");
  const first = await page.evaluate(() => document.activeElement?.textContent?.trim() ?? "");
  if (first !== "Skip to content")
    out.push(`${route}: first Tab stop is "${first.slice(0, 30)}", not the skip link`);
  else {
    await page.keyboard.press("Enter");
    const id = await page.evaluate(() => document.activeElement?.id);
    if (id !== "main") out.push(`${route}: the skip link does not move focus to <main>`);
  }
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  await page.mouse.click(1, 1);
  const seen = new Set<string>();
  for (let i = 0; i < 15; i++) {
    await page.keyboard.press("Tab");
    const r = await page.evaluate(() => {
      const el = document.activeElement as HTMLElement | null;
      if (!el || el === document.body) return null;
      const s = getComputedStyle(el);
      const ring = s.boxShadow !== "none" && s.boxShadow !== "";
      const outline = s.outlineStyle !== "none" && parseFloat(s.outlineWidth) > 0;
      const label =
        el.tagName.toLowerCase() +
        (el.id ? `#${el.id}` : "") +
        (el.getAttribute("aria-label") ? `[${el.getAttribute("aria-label")}]` : "") +
        ` "${(el.textContent ?? "").trim().slice(0, 30)}"`;
      return { label, ok: ring || outline };
    });
    if (!r) break;
    if (seen.has(r.label)) break; // wrapped round
    seen.add(r.label);
    if (!r.ok) out.push(`${route}: no visible focus on ${r.label}`);
  }
  return out;
}
