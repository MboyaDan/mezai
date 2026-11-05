// Case studies (brief 6.1, honesty rules 1 and 4). A record renders only when
// it is published AND names the written client permission behind it. Until
// one exists the index shows an honest "first pilots under way" state and no
// navigation links to it.

export type CaseStudy = {
  slug: string;
  title: string;
  client: string;
  summary: string;
  service: string;
  sections: Array<{ heading: string; body: string[] }>;
  /** Results come with their source, like any stat. */
  results: Array<{ label: string; value: string; source: string }>;
  published: boolean;
  /** Where the client's written permission is filed (docs/content/). */
  permission: { reference: string; grantedOn: string } | null;
};

export const caseStudies: CaseStudy[] = [];

export const caseStudiesPage = {
  eyebrow: "Case studies",
  title: "Case studies",
  empty: {
    title: "First pilots under way.",
    text: "We publish a case study only when the work is done and the client has agreed in writing. Until then, we would rather show you how the work is checked.",
    links: [
      { label: "How quality works", href: "/quality" },
      { label: "Start a pilot project", href: "/contact" },
    ],
  },
};

/** Only these may render, anywhere on the site. */
export function publishedCaseStudies(all: CaseStudy[] = caseStudies): CaseStudy[] {
  return all.filter(
    (c) =>
      c.published &&
      c.permission !== null &&
      c.permission.reference.trim() !== "" &&
      !Number.isNaN(Date.parse(c.permission.grantedOn)) &&
      c.results.every((r) => r.source.trim() !== ""),
  );
}
