// Crops the credited originals in docs/content/images/ (kept out of git, see
// docs/content/image-credits.md for where to download them again) to the
// storyboard shapes and writes web masters to content/images/. next/image
// makes the AVIF and WebP sizes from these at build time.
//
//   node scripts/prepare-photos.mjs
import sharp from "sharp";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const src = (f) => path.join(root, "docs/content/images", f);
const out = (f) => path.join(root, "frontend/content/images", f);

// region is in original pixels: the part of the frame we keep.
const photos = [
  // H1, 4:5: face, laptop and hands; trims empty wall above and the lap below.
  {
    from: "home-people-main.jpg",
    to: "people-linguist.jpg",
    region: { left: 0, top: 300, width: 4000, height: 5000 },
    width: 1600,
  },
  // H2, 3:2: already the right shape; the original is only 1280 px wide.
  {
    from: "home-people-engineer.jpg",
    to: "people-engineer.jpg",
    region: { left: 0, top: 0, width: 1278, height: 852 },
    width: 1278,
  },
  // H3, 3:2 band from a portrait: pen on the chart, hands and stethoscope.
  {
    from: "industry-health.jpg",
    to: "people-clinician.jpg",
    region: { left: 0, top: 3368, width: 4300, height: 2867 },
    width: 1800,
  },
];

for (const p of photos) {
  await sharp(src(p.from))
    .rotate()
    .extract(p.region)
    .resize({ width: p.width })
    .jpeg({ quality: 82, mozjpeg: true }) // sharp drops EXIF (camera, location) by default
    .toFile(out(p.to));
  console.log(p.to);
}
