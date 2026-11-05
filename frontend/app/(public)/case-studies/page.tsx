import type { Metadata } from "next";
import Link from "next/link";
import { Container } from "@/components/public/layout";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { TextLink } from "@/components/ui/text-link";
import { caseStudiesPage, publishedCaseStudies } from "@/content/case-studies";

const published = publishedCaseStudies();

export const metadata: Metadata = {
  title: "Case studies",
  description: caseStudiesPage.empty.text,
  alternates: { canonical: "/case-studies" },
  // Nothing to index until a case study is published.
  robots: published.length === 0 ? { index: false, follow: true } : undefined,
};

export default function CaseStudiesPage() {
  const { empty } = caseStudiesPage;
  return (
    <section aria-labelledby="page-title" className="bg-bg">
      <Container className="py-16 md:py-24">
        <p className="font-mono text-xs tracking-wide text-fg-muted">{caseStudiesPage.eyebrow}</p>
        {published.length === 0 ? (
          <div className="mt-6 grid gap-10 md:grid-cols-12 md:items-center">
            <div className="md:col-span-7">
              <h1
                id="page-title"
                className="font-display text-[2.5rem] leading-[1.05] font-medium tracking-[-0.02em] md:text-display-2"
              >
                {empty.title}
              </h1>
              <p className="mt-5 max-w-xl text-lg text-fg-muted">{empty.text}</p>
              <ul className="mt-8 flex flex-wrap gap-x-8 gap-y-3">
                {empty.links.map((l) => (
                  <li key={l.href}>
                    <TextLink href={l.href}>{l.label} →</TextLink>
                  </li>
                ))}
              </ul>
            </div>
            {/* Scattered: agreement not reached yet. */}
            <div
              aria-hidden="true"
              className="mx-auto w-40 opacity-70 md:col-span-4 md:col-start-9 md:w-full md:max-w-56"
            >
              <SignalLattice rows={5} cols={5} accent={null} state="scattered" />
            </div>
          </div>
        ) : (
          <>
            <h1 id="page-title" className="mt-4 font-display text-[2.5rem] font-medium md:text-display-2">
              {caseStudiesPage.title}
            </h1>
            <ul className="mt-12 grid border-t border-border-strong">
              {published.map((c) => (
                <li key={c.slug} className="border-b border-border py-6">
                  <Link
                    href={`/case-studies/${c.slug}`}
                    className="group grid gap-2 md:grid-cols-12 md:gap-8"
                  >
                    <span className="font-mono text-xs text-fg-muted md:col-span-2">{c.service}</span>
                    <span className="font-display text-2xl font-medium group-hover:underline md:col-span-6">
                      {c.title}
                    </span>
                    <span className="text-fg-muted md:col-span-4">{c.summary}</span>
                  </Link>
                </li>
              ))}
            </ul>
          </>
        )}
      </Container>
    </section>
  );
}
