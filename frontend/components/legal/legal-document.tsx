import type { Metadata } from "next";
import { Container } from "@/components/public/layout";
import { Alert } from "@/components/ui/display";
import { TextLink } from "@/components/ui/text-link";
import { legalBySlug, legalDate, legalDocs, type LegalBlock } from "@/content/legal";

/** Metadata for a legal page; each route under /legal passes its slug. */
export function legalMetadata(slug: string): Metadata {
  const d = legalBySlug(slug);
  if (!d) throw new Error(`unknown legal document ${slug}`);
  return { title: d.short, description: d.summary, alternates: { canonical: `/legal/${d.slug}` } };
}

const anchor = (heading: string) =>
  heading
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");

function Block({ block }: { block: LegalBlock }) {
  if (typeof block === "string") return <p>{block}</p>;
  return (
    <ul className="grid list-disc gap-2 pl-5 marker:text-fg-muted">
      {block.list.map((item) => (
        <li key={item.slice(0, 32)}>{item}</li>
      ))}
    </ul>
  );
}

/** One static route per document keeps navigation instant (no URL params). */
export function LegalDocument({ slug }: { slug: string }) {
  const doc = legalBySlug(slug);
  if (!doc) throw new Error(`unknown legal document ${slug}`);
  const others = legalDocs.filter((d) => d.slug !== doc.slug);

  return (
    <article className="bg-bg">
      <Container className="grid gap-12 py-12 md:py-20 lg:grid-cols-12 lg:gap-x-12">
        <header className="lg:col-span-8">
          <p className="font-mono text-xs tracking-wide text-fg-muted">Legal</p>
          <h1 className="mt-4 font-display text-[2.25rem] leading-[1.05] font-medium tracking-[-0.02em] md:text-display-3">
            {doc.title}
          </h1>
          <p className="mt-5 max-w-[40rem] text-lg text-fg-muted">{doc.summary}</p>
          <p className="mt-5 font-mono text-xs text-fg-muted">
            Version {doc.version} · Last updated <time dateTime={doc.updated}>{legalDate(doc.updated)}</time>
          </p>
          {!doc.approved && (
            <Alert tone="warning" title="Draft, not yet in force" className="mt-8 max-w-[40rem]">
              This draft is waiting for review by a Kenyan advocate. It describes how the platform works
              today, and it may change before launch. The open points are listed at the end.
            </Alert>
          )}
        </header>

        {/* Section index: sticky beside the text on wide screens, a list above it on small ones. */}
        <nav aria-label="On this page" className="lg:col-span-3 lg:row-start-2">
          <div className="lg:sticky lg:top-24">
            <p className="text-sm font-medium">On this page</p>
            <ol className="mt-3 grid gap-2 border-l border-border pl-4 text-sm">
              {doc.sections.map((s) => (
                <li key={s.heading}>
                  <a
                    href={`#${anchor(s.heading)}`}
                    className="text-fg-muted underline-offset-4 hover:text-fg hover:underline"
                  >
                    {s.heading}
                  </a>
                </li>
              ))}
            </ol>
          </div>
        </nav>

        <div className="grid max-w-[42rem] content-start gap-12 lg:col-span-8 lg:col-start-5 lg:row-start-2">
          {doc.sections.map((s) => (
            <section key={s.heading} aria-labelledby={anchor(s.heading)} className="scroll-mt-24">
              <h2 id={anchor(s.heading)} className="font-display text-[1.5rem] leading-tight font-medium">
                {s.heading}
              </h2>
              <div className="mt-4 grid gap-4 text-[1.0625rem] leading-relaxed">
                {s.body.map((b, i) => (
                  <Block key={i} block={b} />
                ))}
              </div>
            </section>
          ))}

          {!doc.approved && doc.openPoints.length > 0 && (
            <section
              aria-labelledby="open-points"
              className="rounded-card border border-border bg-surface p-6"
            >
              <h2 id="open-points" className="font-display text-xl font-medium">
                Open points for legal review
              </h2>
              <ul className="mt-4 grid list-disc gap-2 pl-5 text-fg-muted marker:text-fg-muted">
                {doc.openPoints.map((p) => (
                  <li key={p.slice(0, 32)}>{p}</li>
                ))}
              </ul>
            </section>
          )}

          <nav aria-label="Other legal documents" className="border-t border-border pt-6">
            <p className="text-sm font-medium">Other documents</p>
            <ul className="mt-3 flex flex-wrap gap-x-6 gap-y-2">
              {others.map((d) => (
                <li key={d.slug}>
                  <TextLink href={`/legal/${d.slug}`}>{d.short}</TextLink>
                </li>
              ))}
            </ul>
          </nav>
        </div>
      </Container>
    </article>
  );
}
