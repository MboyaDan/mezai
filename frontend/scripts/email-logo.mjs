// Builds the email logo (D-155): Gmail and Outlook do not show SVG, so emails
// use a PNG of the reversed horizontal logo at three times its display width.
// Run from frontend/: node scripts/email-logo.mjs
// Then set logoWidth/logoHeight in backend/internal/mail/html.go to the
// printed display size.
import { mkdir, readFile } from "node:fs/promises";
import sharp from "sharp";

const src = new URL(
  "../../docs/brand/mezai-logo/mezai-logo-horizontal-reversed-transparent.svg",
  import.meta.url,
);
const out = new URL("../public/brand/email/mezai-logo-email.png", import.meta.url);
const displayWidth = 132;

await mkdir(new URL(".", out), { recursive: true });
const svg = await readFile(src);
const trimmed = await sharp(svg, { density: 144 }).trim().png().toBuffer();
const info = await sharp(trimmed)
  .resize({ width: displayWidth * 3 })
  .png({ compressionLevel: 9, palette: true })
  .toFile(out.pathname);
console.log(`wrote ${out.pathname}: ${info.width}x${info.height} px, ${info.size} bytes`);
console.log(`display size: ${displayWidth}x${Math.round(info.height / 3)}`);
