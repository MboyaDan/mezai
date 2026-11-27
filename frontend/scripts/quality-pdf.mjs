// Prints "How MezAI assures quality" (/print/quality) to a one-page A4 PDF
// (Stage 20a, D-172). Run against a production build whose SITE_URL is the
// public address, because the page prints it:
//
//   SITE_URL=https://mezai.vercel.app npm run build
//   SITE_URL=https://mezai.vercel.app npx next start -p 3000 &
//   npm run pdf:quality
//
// Arguments: [base URL, default http://localhost:3000] [output file].
import { chromium } from "@playwright/test";

const base = process.argv[2] ?? "http://localhost:3000";
const out = process.argv[3] ?? "public/downloads/how-mezai-assures-quality.pdf";

const browser = await chromium.launch();
try {
  const page = await browser.newPage();
  const res = await page.goto(`${base}/print/quality`, { waitUntil: "networkidle" });
  if (!res?.ok()) throw new Error(`/print/quality answered ${res?.status()}`);
  const text = await page.locator("main").innerText();
  if (/localhost|127\.0\.0\.1/.test(text)) {
    throw new Error("the page names localhost: start the site with SITE_URL set to the public address");
  }
  await page.emulateMedia({ media: "print" });
  await page.pdf({ path: out, format: "A4", printBackground: true, preferCSSPageSize: true });
  const pages = await (
    await import("node:fs/promises")
  )
    .readFile(out, "latin1")
    .then((s) => (s.match(/\/Type\s*\/Page[^s]/g) ?? []).length);
  if (pages !== 1) throw new Error(`${out} has ${pages} pages; it must fit on one`);
  console.log(`pdf: ${out} (1 page)`);
} finally {
  await browser.close();
}
