import type { Metadata } from "next";
import { Container, SectionTitle } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { CtaBand } from "@/components/public/cta-band";
import { TextLink } from "@/components/ui/text-link";
import { trustPage, type TrustPoint } from "@/content/trust";

export const metadata: Metadata = {
  title: "Trust and data",
  description: trustPage.intro,
  alternates: { canonical: "/trust" },
  openGraph: { title: titleText(trustPage.title) },
};

/** A ruled list of named statements, as on the About page: text, not cards. */
function Points({ items, numbered = false }: { items: TrustPoint[]; numbered?: boolean }) {
  return (
    <dl className="mt-10 grid border-t border-border-strong md:grid-cols-2">
      {items.map((p, i) => (
        <div key={p.name} className="border-b border-border py-6 md:odd:pr-8 md:even:border-l md:even:pl-8">
          <dt className="flex items-baseline gap-3 font-display text-xl font-medium">
            {numbered && <span className="font-mono text-xs text-fg-muted">0{i + 1}</span>}
            {p.name}
          </dt>
          <dd className="mt-2 text-fg-muted">{p.text}</dd>
        </div>
      ))}
    </dl>
  );
}

export default function TrustPage() {
  const { handling, access, people, commitments, limits, cta } = trustPage;
  // Only commitments ELS has confirmed in writing are shown.
  const confirmed = commitments.items.filter((c) => c.text.trim() !== "");
  return (
    <>
      <PageHeader eyebrow={trustPage.eyebrow} title={trustPage.title} intro={trustPage.intro} />

      <section aria-labelledby="handling-title" className="bg-bg">
        <Container className="py-16 md:py-24">
          <SectionTitle id="handling-title" className="max-w-3xl lg:text-display-3">
            {handling.title}
          </SectionTitle>
          <Points items={handling.points} />
        </Container>
      </section>

      <section aria-labelledby="access-title" className="border-t border-border bg-surface">
        <Container className="py-16 md:py-24">
          <SectionTitle id="access-title" className="max-w-3xl lg:text-display-3">
            {access.title}
          </SectionTitle>
          <Points items={access.points} />
        </Container>
      </section>

      <section aria-labelledby="people-title" className="border-t border-border bg-bg">
        <Container className="py-16 md:py-24">
          <SectionTitle id="people-title" className="max-w-3xl lg:text-display-3">
            {people.title}
          </SectionTitle>
          <p className="mt-5 max-w-2xl text-lg text-fg-muted">{people.intro}</p>
          <Points items={people.steps} numbered />
          <p className="mt-8 max-w-2xl text-fg-muted">{people.scale}</p>
          <TextLink href={people.link.href} className="mt-4 inline-block">
            {people.link.label}
          </TextLink>
        </Container>
      </section>

      <section aria-labelledby="limits-title" className="border-t border-border bg-surface">
        <Container className="grid gap-10 py-16 md:grid-cols-12 md:py-24">
          {confirmed.length > 0 && (
            <div className="md:col-span-6">
              <h2 className="font-display text-[2rem] leading-[1.08] font-medium md:text-display-3">
                {commitments.title}
              </h2>
              <dl className="mt-6 grid gap-5">
                {confirmed.map((c) => (
                  <div key={c.name} className="border-t border-border pt-4">
                    <dt className="font-display text-lg font-medium">{c.name}</dt>
                    <dd className="mt-1 text-fg-muted">{c.text}</dd>
                  </div>
                ))}
              </dl>
            </div>
          )}
          <div className={confirmed.length > 0 ? "md:col-span-5 md:col-start-8" : "md:col-span-8"}>
            <h2
              id="limits-title"
              className="font-display text-[2rem] leading-[1.08] font-medium md:text-display-3"
            >
              {limits.title}
            </h2>
            <p className="mt-5 text-lg text-fg-muted">{limits.text}</p>
          </div>
        </Container>
      </section>

      <CtaBand title="Ask us anything about your data." primary={cta.primary} secondary={cta.secondary} />
    </>
  );
}
