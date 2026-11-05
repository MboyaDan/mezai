import type { Metadata } from "next";
import Link from "next/link";
import { Container } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { CtaBand } from "@/components/public/cta-band";
import { IllustrativePhoto } from "@/components/public/photo";
import { industriesPage } from "@/content/industries";
import { servicesPage } from "@/content/services";
import education from "@/content/images/industry-education.jpg";

export const metadata: Metadata = {
  title: "Industries",
  description: industriesPage.intro,
  alternates: { canonical: "/industries" },
  openGraph: { title: titleText(industriesPage.title) },
};

const serviceByKey = new Map(servicesPage.items.map((s) => [s.key, s]));

export default function IndustriesPage() {
  const { items, photo, cta } = industriesPage;
  return (
    <>
      <PageHeader
        eyebrow={industriesPage.eyebrow}
        title={industriesPage.title}
        intro={industriesPage.intro}
        aside={
          <ul className="grid gap-1 border-t border-border font-mono text-xs text-fg-muted">
            {items.map((it, i) => (
              <li key={it.id} className="border-b border-border">
                <a href={`#${it.id}`} className="flex min-h-11 items-center gap-3 hover:text-fg">
                  <span>0{i + 1}</span>
                  <span className="font-sans text-sm text-fg">{it.name}</span>
                </a>
              </li>
            ))}
          </ul>
        }
      />

      <div className="bg-bg">
        <Container className="py-14 md:py-24">
          {items.map((it, i) => {
            const hasPhoto = it.id === photo.industry;
            return (
              <section
                key={it.id}
                id={it.id}
                aria-labelledby={`${it.id}-title`}
                className="grid scroll-mt-24 gap-6 border-t border-border-strong py-10 md:grid-cols-12 md:gap-x-10 md:py-14"
              >
                <p className="font-mono text-xs text-fg-muted md:col-span-1">0{i + 1}</p>
                <div className="md:col-span-5">
                  <h2
                    id={`${it.id}-title`}
                    className="font-display text-[1.75rem] leading-[1.1] font-medium tracking-[-0.01em] md:text-[2.25rem]"
                  >
                    {it.name}
                  </h2>
                  <p className="mt-4 text-lg text-fg-muted">{it.needs}</p>
                </div>
                <div className="grid content-start gap-6 md:col-span-6">
                  {hasPhoto && (
                    <IllustrativePhoto
                      src={education}
                      alt={photo.alt}
                      sizes="(min-width: 768px) 45vw, 100vw"
                      bracket
                    />
                  )}
                  <div>
                    <h3 className="font-mono text-xs tracking-wide text-fg-muted">Typical work</h3>
                    <ul className="mt-3 grid gap-2">
                      {it.examples.map((e) => (
                        <li key={e} className="border-t border-border pt-2">
                          {e}
                        </li>
                      ))}
                    </ul>
                  </div>
                  <div>
                    <h3 className="font-mono text-xs tracking-wide text-fg-muted">Services that fit</h3>
                    <ul className="mt-3 flex flex-wrap gap-2">
                      {it.services.map((key) => {
                        const s = serviceByKey.get(key)!;
                        return (
                          <li key={key}>
                            <Link
                              href={`/services#${s.id}`}
                              className="inline-flex min-h-9 items-center rounded-full border border-border-strong px-3 text-sm hover:bg-inverse-bg hover:text-inverse-fg focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
                            >
                              {s.name}
                            </Link>
                          </li>
                        );
                      })}
                    </ul>
                  </div>
                </div>
              </section>
            );
          })}
        </Container>
      </div>

      <CtaBand title={cta.title} text={cta.text} primary={cta.primary} />
    </>
  );
}
