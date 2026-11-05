import type { Metadata } from "next";
import { Check } from "lucide-react";
import { Container, Eyebrow } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { CtaBand } from "@/components/public/cta-band";
import { PageMotion } from "@/components/public/page-motion";
import { serviceDemo } from "@/components/home/service-demos";
import { TextLink } from "@/components/ui/text-link";
import { languages, servicesPage, type Service } from "@/content/services";
import { cn } from "@/lib/cn";

export const metadata: Metadata = {
  title: "Services",
  description: servicesPage.intro,
  alternates: { canonical: "/services" },
  openGraph: { title: titleText(servicesPage.title) },
};

function List({ title, items, ordered = false }: { title: string; items: string[]; ordered?: boolean }) {
  const Tag = ordered ? "ol" : "ul";
  return (
    <div>
      <h3 className="font-mono text-xs tracking-wide text-fg-muted">{title}</h3>
      <Tag className="mt-3 grid gap-3">
        {items.map((item, i) => (
          <li key={item} className="grid grid-cols-[1.5rem_1fr] items-baseline gap-1 text-[0.9375rem]">
            {ordered ? (
              <span className="font-mono text-xs text-fg-muted">{i + 1}.</span>
            ) : (
              <Check aria-hidden="true" className="size-3.5 translate-y-0.5 text-accent-fg" />
            )}
            <span>{item}</span>
          </li>
        ))}
      </Tag>
    </div>
  );
}

function ServiceSection({ s, n }: { s: Service; n: number }) {
  const Demo = serviceDemo[s.key];
  return (
    <section
      id={s.id}
      aria-labelledby={`${s.id}-title`}
      className="scroll-mt-24 border-t border-border-strong pt-8 pb-16 md:pb-24"
    >
      <div className="grid gap-8 lg:grid-cols-9 lg:gap-x-10" data-reveal>
        <div className="lg:col-span-5">
          <p className="font-mono text-xs text-fg-muted">0{n}</p>
          <h2
            id={`${s.id}-title`}
            className="mt-3 font-display text-[1.75rem] leading-[1.1] font-medium tracking-[-0.01em] md:text-[2.25rem]"
          >
            {s.name}
          </h2>
          <p className="mt-4 text-lg text-fg-muted">{s.summary}</p>
        </div>
        <div className="lg:col-span-4 lg:pt-8">
          <Demo />
        </div>
      </div>
      <div className="mt-10 grid gap-8 md:grid-cols-2 lg:grid-cols-9 lg:gap-x-10">
        <div className="lg:col-span-3">
          <List title="What contributors do" items={s.task} ordered />
        </div>
        <div className="lg:col-span-3">
          <List title="What you receive" items={s.deliver} />
        </div>
        <div className="md:col-span-2 lg:col-span-3">
          <List title="How it is checked" items={s.checks} />
        </div>
      </div>
      <p className="mt-8 border-l-2 border-laterite pl-4 text-[0.9375rem]">
        <span className="font-medium">Good for: </span>
        <span className="text-fg-muted">{s.goodFor}</span>
      </p>
    </section>
  );
}

export default function ServicesPage() {
  const { items, common } = servicesPage;
  return (
    <>
      <PageHeader eyebrow={servicesPage.eyebrow} title={servicesPage.title} intro={servicesPage.intro} />

      <div className="bg-bg">
        <Container className="grid gap-10 py-14 md:py-20 lg:grid-cols-12 lg:gap-x-12">
          {/* Sticky index on wide screens; a wrapped list of jump links on small ones. */}
          <nav aria-label="Services on this page" className="lg:col-span-3">
            <div className="lg:sticky lg:top-24">
              <Eyebrow>On this page</Eyebrow>
              <ol className="mt-4 flex flex-wrap gap-x-6 gap-y-2 lg:grid lg:gap-0 lg:border-t lg:border-border">
                {[
                  ...items.map((s) => ({ id: s.id, name: s.name })),
                  { id: "languages", name: languages.title },
                ].map((s, i) => (
                  <li key={s.id} className="lg:border-b lg:border-border">
                    <a
                      href={`#${s.id}`}
                      className="inline-flex min-h-11 items-center gap-3 text-[0.9375rem] hover:text-accent-fg lg:flex lg:py-3"
                    >
                      <span className="hidden font-mono text-xs text-fg-muted lg:inline">0{i + 1}</span>
                      {s.name}
                    </a>
                  </li>
                ))}
              </ol>
            </div>
          </nav>

          <div className="lg:col-span-9">
            {items.map((s, i) => (
              <ServiceSection key={s.id} s={s} n={i + 1} />
            ))}

            <section
              id="languages"
              aria-labelledby="languages-title"
              className="scroll-mt-24 border-t border-border-strong pt-8"
            >
              <div className="grid gap-8 lg:grid-cols-9 lg:gap-x-10">
                <div className="lg:col-span-4">
                  <p className="font-mono text-xs text-fg-muted">0{items.length + 1}</p>
                  <h2
                    id="languages-title"
                    className="mt-3 font-display text-[1.75rem] leading-[1.1] font-medium md:text-[2.25rem]"
                  >
                    {languages.title}
                  </h2>
                  <p className="mt-4 text-fg-muted">{languages.intro}</p>
                </div>
                <div className="lg:col-span-5">
                  <table className="w-full border-collapse text-left text-[0.9375rem]">
                    <caption className="sr-only">Languages we can staff and check today</caption>
                    <thead>
                      <tr className="border-b border-border-strong font-mono text-xs text-fg-muted">
                        <th scope="col" className="py-3 pr-4 font-normal">
                          Language
                        </th>
                        <th scope="col" className="py-3 font-normal">
                          Status
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {languages.rows.map((r) => (
                        <tr key={r.name} className="border-b border-border">
                          <th scope="row" className="py-3 pr-4 font-medium">
                            {r.name}
                          </th>
                          <td className="py-3">
                            <span className="inline-flex items-center gap-2">
                              <span
                                aria-hidden="true"
                                className={cn(
                                  "size-2 rounded-full",
                                  r.status === "available"
                                    ? "bg-accent-fg"
                                    : "border border-fg-muted bg-transparent",
                                )}
                              />
                              {languages.statusLabel[r.status]}
                            </span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                  <p className="mt-4 text-sm text-fg-muted">{languages.basis}</p>
                  <TextLink href={languages.cta.href} tone="accent" className="mt-4 inline-block">
                    {languages.cta.label} →
                  </TextLink>
                </div>
              </div>
            </section>
          </div>
        </Container>
      </div>

      <section aria-labelledby="common-title" className="border-t border-border bg-surface">
        <Container className="grid gap-8 py-14 md:grid-cols-12 md:py-20">
          <h2
            id="common-title"
            className="font-display text-[1.75rem] font-medium md:col-span-4 md:text-[2rem]"
          >
            {common.title}
          </h2>
          <ol className="grid gap-6 md:col-span-7 md:col-start-6" data-reveal-group>
            {common.items.map((item, i) => (
              <li key={item} className="grid grid-cols-[2rem_1fr] items-baseline border-t border-border pt-4">
                <span className="font-mono text-sm text-fg-muted">0{i + 1}</span>
                <span>{item}</span>
              </li>
            ))}
          </ol>
          <div className="md:col-span-7 md:col-start-6">
            <TextLink href={common.link.href}>{common.link.label} →</TextLink>
          </div>
        </Container>
      </section>

      <CtaBand
        title="Start with a small pilot."
        text="Your data, your rubric, and the full quality report at the end."
        primary={{ label: "Start a pilot project", href: "/contact" }}
      />
      <PageMotion />
    </>
  );
}
