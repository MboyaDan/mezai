// Where the site lives and whether search engines may index it. The domain
// waits on DG-01, so it comes from the environment.
export const siteUrl = (process.env.SITE_URL ?? "http://localhost:3000").replace(/\/$/, "");

/** Only production sets SITE_INDEXABLE=1; local and staging stay out of search. */
export const indexable = process.env.SITE_INDEXABLE === "1";

/** Public pages that exist today; later stages add theirs (Stage 6). */
export const publicRoutes = ["/"] as const;
