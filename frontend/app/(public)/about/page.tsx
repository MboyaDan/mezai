import type { Metadata } from "next";
import { Container, SectionTitle } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { CtaBand } from "@/components/public/cta-band";
import { IllustrativePhoto } from "@/components/public/photo";
import { TextLink } from "@/components/ui/text-link";
import { aboutPage } from "@/content/about";
import { brand } from "@/lib/brand";
import table from "@/content/images/about-table.jpg";

export const metadata: Metadata = {
  title: "About",
  description: aboutPage.story[0],
  alternates: { canonical: "/about" },
  openGraph: { title: titleText(aboutPage.title) },
};

export default function AboutPage() {
  const { story, beliefs, pilot, entity, photo, cta } = aboutPage;
  return (
    <>
      <PageHeader eyebrow={aboutPage.eyebrow} title={aboutPage.title} intro={aboutPage.intro} />

      <section aria-label="Our story" className="bg-bg">
        <Container className="grid gap-12 py-16 md:py-24 lg:grid-cols-12 lg:gap-x-12">
          <IllustrativePhoto
            src={table}
            alt={photo.alt}
            sizes="(min-width: 1024px) 55vw, 100vw"
            className="lg:col-span-7"
            bracket
          />
          <div className="grid content-start gap-5 text-lg lg:col-span-5 lg:pt-6">
            {story.map((p) => (
              <p key={p.slice(0, 24)}>{p}</p>
            ))}
          </div>
        </Container>
      </section>

      <section aria-labelledby="beliefs-title" className="border-t border-border bg-bg">
        <Container className="py-16 md:py-24">
          <SectionTitle id="beliefs-title" className="max-w-3xl lg:text-display-3">
            {beliefs.title}
          </SectionTitle>
          {/* Three statements in a ruled row: text, not cards. */}
          <dl className="mt-12 grid border-t border-border-strong md:grid-cols-3">
            {beliefs.items.map((b, i) => (
              <div
                key={b.name}
                className="border-b border-border py-6 md:border-b-0 md:px-8 md:first:pl-0 md:[&+&]:border-l"
              >
                <dt className="flex items-baseline gap-3 font-display text-xl font-medium">
                  <span className="font-mono text-xs text-fg-muted">0{i + 1}</span>
                  {b.name}
                </dt>
                <dd className="mt-2 text-fg-muted">{b.text}</dd>
              </div>
            ))}
          </dl>
        </Container>
      </section>

      <section aria-labelledby="pilot-title" className="border-t border-border bg-surface">
        <Container className="grid gap-10 py-16 md:grid-cols-12 md:py-24">
          <div className="md:col-span-6">
            <h2
              id="pilot-title"
              className="font-display text-[2rem] leading-[1.08] font-medium md:text-display-3"
            >
              {pilot.title}
            </h2>
            <p className="mt-5 text-lg text-fg-muted">{pilot.text}</p>
          </div>
          <div className="grid content-start gap-6 md:col-span-5 md:col-start-8">
            <div className="border-t border-border pt-5">
              <h3 className="font-display text-xl font-medium">{entity.title}</h3>
              <p className="mt-2 text-fg-muted">{entity.text}</p>
            </div>
            {/* Contact details come from docs/content/ (Q2); hidden while empty. */}
            {brand.contactEmail && (
              <div className="border-t border-border pt-5">
                <h3 className="font-display text-xl font-medium">Email</h3>
                <TextLink href={`mailto:${brand.contactEmail}`} className="mt-2 inline-block">
                  {brand.contactEmail}
                </TextLink>
              </div>
            )}
          </div>
        </Container>
      </section>

      <CtaBand title="Sit down at the table." primary={cta.primary} secondary={cta.secondary} />
    </>
  );
}
