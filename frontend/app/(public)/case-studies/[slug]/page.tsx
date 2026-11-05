import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { Container } from "@/components/public/layout";
import { CtaBand } from "@/components/public/cta-band";
import { Breadcrumb } from "@/components/ui/display";
import { publishedCaseStudies } from "@/content/case-studies";

// Only published case studies with written permission exist as pages; any
// other slug answers 404. Cache Components needs at least one param, so with
// none published a placeholder is generated (and also answers 404).
const PLACEHOLDER = "__none__";

export function generateStaticParams() {
  const slugs = publishedCaseStudies().map((c) => ({ slug: c.slug }));
  return slugs.length > 0 ? slugs : [{ slug: PLACEHOLDER }];
}

const find = (slug: string) => publishedCaseStudies().find((c) => c.slug === slug);

export async function generateMetadata({ params }: PageProps<"/case-studies/[slug]">): Promise<Metadata> {
  const c = find((await params).slug);
  return c
    ? { title: c.title, description: c.summary, alternates: { canonical: `/case-studies/${c.slug}` } }
    : { robots: { index: false } };
}

export default async function CaseStudyPage({ params }: PageProps<"/case-studies/[slug]">) {
  const c = find((await params).slug);
  if (!c) notFound();
  return (
    <>
      <article className="bg-bg">
        <Container className="py-12 md:py-20">
          <Breadcrumb items={[{ label: "Case studies", href: "/case-studies" }, { label: c.title }]} />
          <p className="mt-8 font-mono text-xs text-fg-muted">
            {c.service} · {c.client}
          </p>
          <h1 className="mt-4 max-w-4xl font-display text-[2.5rem] leading-[1.05] font-medium md:text-display-2">
            {c.title}
          </h1>
          <p className="mt-5 max-w-2xl text-lg text-fg-muted">{c.summary}</p>
          {c.results.length > 0 && (
            <dl className="mt-12 grid gap-6 border-t border-border-strong pt-6 sm:grid-cols-3">
              {c.results.map((r) => (
                <div key={r.label} data-stat>
                  <dt className="text-sm text-fg-muted">{r.label}</dt>
                  <dd className="font-mono text-3xl tabular">{r.value}</dd>
                  <dd className="mt-1 text-xs text-fg-muted">Source: {r.source}</dd>
                </div>
              ))}
            </dl>
          )}
          <div className="mt-12 grid max-w-[68ch] gap-10">
            {c.sections.map((s) => (
              <section key={s.heading}>
                <h2 className="font-display text-2xl font-medium">{s.heading}</h2>
                {s.body.map((p) => (
                  <p key={p.slice(0, 24)} className="mt-4">
                    {p}
                  </p>
                ))}
              </section>
            ))}
          </div>
        </Container>
      </article>
      <CtaBand title="Run a pilot with us." primary={{ label: "Start a pilot project", href: "/contact" }} />
    </>
  );
}
