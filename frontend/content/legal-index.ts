// Titles, versions and dates of the legal documents, without their text.
// The footer, sitemap and sign-up consent need only these, so they import
// this file and the full text (content/legal.ts) loads only on the legal
// pages (Stage 18a: it was 14 KB of JavaScript on every page).

export type LegalMeta = {
  slug: string;
  title: string;
  /** Shown in the footer and page title. */
  short: string;
  version: string;
  /** ISO date of the last change. */
  updated: string;
  /** False until the advocate review in DG-05 approves this version. */
  approved: boolean;
};

export const legalIndex: LegalMeta[] = [
  {
    slug: "privacy",
    title: "Privacy notice",
    short: "Privacy notice",
    version: "0.4",
    updated: "2026-11-16",
    approved: false,
  },
  {
    slug: "terms",
    title: "Terms of use",
    short: "Terms of use",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
  },
  {
    slug: "contributor-terms",
    title: "Contributor terms",
    short: "Contributor terms",
    version: "0.4",
    updated: "2026-11-16",
    approved: false,
  },
  {
    slug: "acceptable-use",
    title: "Acceptable use policy for contributors",
    short: "Acceptable use",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
  },
  {
    slug: "cookies",
    title: "Cookie notice",
    short: "Cookies",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
  },
];

export function legalMeta(slug: string): LegalMeta {
  const m = legalIndex.find((d) => d.slug === slug);
  if (!m) throw new Error(`no legal document ${slug}`);
  return m;
}
