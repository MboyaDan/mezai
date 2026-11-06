import { legalDocs } from "@/content/legal";

// Where the site lives and whether search engines may index it. The domain
// waits on DG-01, so it comes from the environment.
export const siteUrl = (process.env.SITE_URL ?? "http://localhost:3000").replace(/\/$/, "");

/** Only production sets SITE_INDEXABLE=1; local and staging stay out of search. */
export const indexable = process.env.SITE_INDEXABLE === "1";

/** Public pages that exist today (the sitemap lists these). */
export const publicRoutes = [
  "/",
  "/services",
  "/quality",
  "/industries",
  "/contributors",
  "/about",
  "/contact",
  ...legalDocs.map((d) => `/legal/${d.slug}` as const),
] as const;
