import { expect, test } from "@playwright/test";

// Stage 17a (D-138): every page carries the security headers, and the
// content security policy blocks nothing the pages need.

const pages = [
  "/",
  "/services",
  "/quality",
  "/contributors",
  "/contact",
  "/legal/privacy",
  "/sign-in",
  "/join",
  "/design",
];

test("pages send the security headers", async ({ request }) => {
  for (const path of ["/", "/login", "/no-such-page"]) {
    const res = await request.get(path);
    const h = res.headers();
    expect(h["x-frame-options"], path).toBe("DENY");
    expect(h["x-content-type-options"], path).toBe("nosniff");
    expect(h["referrer-policy"], path).toBe("strict-origin-when-cross-origin");
    expect(h["content-security-policy"], path).toContain("frame-ancestors 'none'");
    expect(h["content-security-policy"], path).toContain("default-src 'self'");
    expect(h["x-powered-by"], path).toBeUndefined();
  }
});

test("the content security policy blocks nothing the pages use", async ({ page }) => {
  const violations: string[] = [];
  page.on("console", (m) => {
    if (/Content Security Policy|Content-Security-Policy/i.test(m.text())) violations.push(m.text());
  });
  await page.addInitScript(() => {
    document.addEventListener("securitypolicyviolation", (e) => {
      console.error(`Content Security Policy violation: ${e.violatedDirective} ${e.blockedURI}`);
    });
  });
  for (const path of pages) {
    await page.goto(path);
    await page.waitForLoadState("networkidle");
    // Scroll through, so lazy sections and scroll motion load too.
    await page.evaluate(async () => {
      for (let y = 0; y < document.body.scrollHeight; y += 600) {
        window.scrollTo(0, y);
        await new Promise((r) => setTimeout(r, 40));
      }
    });
  }
  expect(violations).toEqual([]);
});

test("a page cannot be put in a frame", async ({ page }) => {
  await page.goto("/");
  const framed = await page.evaluate(async () => {
    const f = document.createElement("iframe");
    f.src = "/login";
    document.body.appendChild(f);
    await new Promise((r) => setTimeout(r, 1500));
    try {
      return (f.contentDocument?.body?.innerText ?? "").length > 0;
    } catch {
      return false;
    }
  });
  expect(framed).toBe(false);
});
