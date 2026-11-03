import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const pages = ["/design", "/design/shells/public", "/design/shells/contributor", "/design/shells/workspace"];

for (const path of pages) {
  test(`axe: no critical or serious issues on ${path}`, async ({ page }) => {
    await page.goto(path);
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "wcag22aa"])
      .analyze();
    const blocking = results.violations.filter((v) => v.impact === "critical" || v.impact === "serious");
    expect(blocking.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`)).toEqual([]);
  });
}

test.describe("360 px", () => {
  test.use({ viewport: { width: 360, height: 780 } });
  for (const path of pages) {
    test(`${path} has no horizontal scroll`, async ({ page }) => {
      await page.goto(path);
      const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
      expect(overflow).toBeLessThanOrEqual(0);
    });
  }

  test("workspace navigation opens as a drawer and closes with Escape", async ({ page }) => {
    await page.goto("/design/shells/workspace");
    const open = page.getByRole("button", { name: "Open navigation" });
    await open.click();
    const drawer = page.getByRole("dialog", { name: "Navigation" });
    await expect(drawer.getByRole("link", { name: "Projects" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(drawer).toBeHidden();
    await expect(open).toBeFocused();
  });
});

test("dialog traps focus, closes with Escape and returns focus", async ({ page }) => {
  await page.goto("/design#overlays");
  const trigger = page.getByRole("button", { name: "Open dialog" });
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "Reject this submission?" });
  await expect(dialog).toBeVisible();
  for (let i = 0; i < 6; i++) {
    await page.keyboard.press("Tab");
    expect(await dialog.evaluate((d) => d.contains(document.activeElement))).toBe(true);
  }
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(trigger).toBeFocused();
});

test("tabs move with arrow keys", async ({ page }) => {
  await page.goto("/design#disclosure");
  const first = page.getByRole("tab", { name: "Contributor view" }).first();
  await first.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Client view" }).first()).toBeFocused();
  await expect(page.getByRole("tab", { name: "Client view" }).first()).toHaveAttribute(
    "aria-selected",
    "true",
  );
});

test("table sorts by column and announces the order", async ({ page }) => {
  await page.goto("/design#data");
  const table = page.getByRole("table", { name: "Submissions (Sample, light)" });
  const firstItem = () => table.locator("tbody tr").first().locator("td").first();
  await expect(firstItem()).toHaveText("ITM-0023");
  const header = table.getByRole("columnheader", { name: /Gold acc/ });
  await header.getByRole("button").click();
  await expect(header).toHaveAttribute("aria-sort", "ascending");
  await expect(firstItem()).toHaveText("ITM-0023");
  await header.getByRole("button").click();
  await expect(header).toHaveAttribute("aria-sort", "descending");
  await expect(firstItem()).toHaveText("ITM-0107");
  // Rows without a gold score stay last in both directions.
  await expect(table.locator("tbody tr").last().locator("td").first()).toHaveText("ITM-0310");
});

test("pagination marks the current page", async ({ page }) => {
  await page.goto("/design#data");
  const nav = page.getByRole("navigation", { name: "Pagination (light)" });
  // A "Page 4" link on desktop, a compact "4 / 12" under 640 px.
  await expect(nav.locator('[aria-current="page"]:visible')).toContainText("4");
});

test.describe("reduced motion", () => {
  test.use({ reducedMotion: "reduce" });

  test("reveals render in their final state and charts do not animate", async ({ page }) => {
    await page.goto("/design#motion");
    const item = page.getByRole("listitem").filter({ hasText: /^Overlap$/ });
    await expect(item).toHaveCSS("opacity", "1");
    await expect(page.getByText("Fades in with a 16 px rise")).toBeVisible();
    const bar = page.locator(".chart-bar").first();
    const duration = await bar.evaluate((el) => parseFloat(getComputedStyle(el).animationDuration));
    expect(duration).toBeLessThan(0.001);
  });

  test("lattice renders aligned immediately", async ({ page }) => {
    await page.goto("/");
    const angle = await page
      .locator(".signal-lattice line")
      .first()
      .evaluate((l) => getComputedStyle(l).transform);
    expect(["none", "matrix(1, 0, 0, 1, 0, 0)"]).toContain(angle);
  });
});
