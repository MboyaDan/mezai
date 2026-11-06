import { expect, test } from "@playwright/test";
import { publicRoutes } from "../../lib/site";
import { legalDocs } from "../../content/legal";

// AC-03 on the rendered site: every public route, plus the 404 page.
// content/honesty.test.ts runs the same rules on the source in CI's unit job.
const routes = [...publicRoutes, "/case-studies", "/this-page-does-not-exist"];

const forbidden =
  /\b(SOC ?2|ISO ?27001|HIPAA|GDPR[- ](certified|compliant)|certified|accredited|trusted by|our clients include|as seen in|testimonials?|award[- ]winning)\b/i;

for (const path of routes) {
  test(`honesty: ${path}`, async ({ page }) => {
    await page.goto(path);
    const text = await page.locator("body").innerText();
    expect(text.match(forbidden)?.[0] ?? null).toBeNull();

    for (const stat of await page.locator("[data-stat]").all()) {
      await expect(stat).toContainText("Source:");
    }
    for (const demo of await page.locator("[data-demo]").all()) {
      await expect(demo).toContainText(/Demonstration|Worked example/);
    }
    await expect(page.locator("blockquote, q")).toHaveCount(0);

    // Photos of people are only ever illustrative and credited (rule 4, DESIGN.md 8).
    for (const img of await page.locator("main img").all()) {
      const figure = img.locator("xpath=ancestor::figure[1]");
      await expect(figure, `photo without an illustrative caption on ${path}`).toContainText("Illustrative");
    }
  });
}

test("unknown pages answer 404 with the site shell", async ({ page }) => {
  const res = await page.goto("/this-page-does-not-exist");
  expect(res?.status()).toBe(404);
  await expect(page.getByRole("heading", { level: 1, name: "Nothing lines up here." })).toBeVisible();
  await expect(page.locator("footer")).toBeVisible();
});

test("legal drafts say so and show their version", async ({ page }) => {
  for (const doc of legalDocs) {
    await page.goto(`/legal/${doc.slug}`);
    await expect(page.getByText(`Version ${doc.version}`)).toBeVisible();
    if (!doc.approved) await expect(page.getByText("Draft, not yet in force")).toBeVisible();
  }
  await page.goto("/");
  for (const doc of legalDocs) {
    await expect(page.locator(`footer a[href="/legal/${doc.slug}"]`)).toHaveCount(1);
  }
});

test.describe("enquiry form", () => {
  test("checks fields before sending and focuses the first problem", async ({ page }) => {
    let calls = 0;
    await page.route("**/api/v1/enquiries", (r) => {
      calls++;
      return r.fulfill({ status: 202, json: { status: "received" } });
    });
    await page.goto("/contact");
    await page.getByRole("button", { name: "Send enquiry" }).click();
    await expect(page.getByText("4 fields need attention.")).toBeVisible();
    await expect(page.getByLabel(/Your name/)).toBeFocused();
    await expect(page.getByText("Enter your name.")).toBeVisible();
    expect(calls).toBe(0);
  });

  test("sends the brief's fields, hides the honeypot and confirms", async ({ page }) => {
    let body: Record<string, string> = {};
    await page.route("**/api/v1/enquiries", (r) => {
      body = r.request().postDataJSON();
      return r.fulfill({ status: 202, json: { status: "received" } });
    });
    await page.goto("/contact");
    await expect(page.locator('input[name="website"]')).not.toBeInViewport();
    await expect(page.locator('input[name="website"]')).toHaveAttribute("tabindex", "-1");

    await page.getByLabel(/Your name/).fill("Sample Buyer");
    await page.getByLabel(/Organisation/).fill("Sample Lab (Sample)");
    await page.getByLabel(/Work email/).fill("buyer@sample.invalid");
    await page.getByLabel(/Project type/).selectOption("ai-evaluation");
    await page.getByLabel(/Languages/).fill("Kiswahili, English");
    await page.getByLabel(/Volume/).selectOption("1k-10k");
    await page.getByLabel(/Timeline/).selectOption("1-month");
    await page.getByLabel(/Message/).fill("Pairwise comparison of two model versions.");
    await page.getByRole("button", { name: "Send enquiry" }).click();

    await expect(page.getByRole("heading", { name: "Enquiry received" })).toBeVisible();
    expect(body).toMatchObject({
      name: "Sample Buyer",
      organisation: "Sample Lab (Sample)",
      email: "buyer@sample.invalid",
      project_type: "ai-evaluation",
      languages: "Kiswahili, English",
      volume: "1k-10k",
      timeline: "1-month",
      website: "",
    });
  });

  test("shows the API's field errors and rate limit", async ({ page }) => {
    let n = 0;
    await page.route("**/api/v1/enquiries", (r) =>
      r.fulfill(
        n++ === 0
          ? { status: 422, json: { error: "invalid_enquiry", fields: { email: "invalid" } } }
          : { status: 429, json: { error: "too_many_requests" } },
      ),
    );
    await page.goto("/contact");
    await page.getByLabel(/Your name/).fill("Sample Buyer");
    await page.getByLabel(/Organisation/).fill("Sample Lab (Sample)");
    await page.getByLabel(/Work email/).fill("buyer@sample.invalid");
    await page.getByLabel(/Project type/).selectOption("expert-review");
    await page.getByRole("button", { name: "Send enquiry" }).click();
    await expect(page.getByText("Enter an email address like name@example.com.")).toBeVisible();
    await page.getByRole("button", { name: "Send enquiry" }).click();
    await expect(page.getByText(/Too many enquiries/)).toBeVisible();
  });
});
