import { Plus } from "lucide-react";
import { TextLink } from "@/components/ui/text-link";
import { services } from "@/content/home";
import { Accent, Container, Eyebrow, SectionTitle } from "@/components/public/layout";
import { serviceDemo } from "./service-demos";

/** Frame 3. Rows on desktop; an accordion on mobile (DESIGN.md section 10). */
export function Services() {
  return (
    <section id="services" aria-labelledby="services-title" className="bg-bg">
      <Container className="py-20 md:py-32">
        <div className="grid gap-4 md:grid-cols-12" data-reveal>
          <Eyebrow className="md:col-span-3 md:pt-4">{services.eyebrow}</Eyebrow>
          <SectionTitle id="services-title" className="md:col-span-8 md:col-start-4">
            {services.title.lead} <Accent>{services.title.accent}</Accent> {services.title.tail}
          </SectionTitle>
        </div>

        {/* Desktop: all rows open, the first framed by the bracket. */}
        <ol className="svc-list mt-16 hidden border-t border-border-strong md:block" data-reveal-group>
          {services.items.map((s, i) => {
            const Demo = serviceDemo[s.key];
            return (
              <li
                key={s.key}
                id={`service-${s.key}`}
                className="svc-row relative scroll-mt-24 border-b border-border"
              >
                <span
                  aria-hidden="true"
                  className="svc-bracket pointer-events-none absolute -top-px -left-3 size-3.5 border-t border-l border-fg"
                />
                <span
                  aria-hidden="true"
                  className="svc-bracket pointer-events-none absolute -right-3 -bottom-px size-3.5 border-r border-b border-fg"
                />
                <div className="grid grid-cols-12 items-start gap-6 py-8">
                  <span className="col-span-1 font-mono text-xs text-fg-muted">0{i + 1}</span>
                  <h3 className="col-span-4 font-display text-2xl font-medium">{s.name}</h3>
                  <p className="col-span-3 text-fg-muted">{s.line}</p>
                  <div className="col-span-4">
                    <Demo />
                  </div>
                </div>
              </li>
            );
          })}
        </ol>

        {/* Mobile: native disclosure, works without JavaScript. */}
        <div className="mt-10 border-t border-border-strong md:hidden">
          {services.items.map((s, i) => {
            const Demo = serviceDemo[s.key];
            return (
              <details key={s.key} open={i === 0} className="group border-b border-border">
                <summary className="flex min-h-14 cursor-pointer list-none items-center gap-4 py-4 [&::-webkit-details-marker]:hidden">
                  <span className="font-mono text-xs text-fg-muted">0{i + 1}</span>
                  <span className="flex-1 font-display text-xl font-medium">{s.name}</span>
                  <Plus
                    aria-hidden="true"
                    className="size-4 text-fg-muted transition-transform group-open:rotate-45"
                  />
                </summary>
                <div className="grid gap-4 pb-6">
                  <p className="text-fg-muted">{s.line}</p>
                  <Demo />
                </div>
              </details>
            );
          })}
        </div>

        <div className="mt-12 flex flex-col gap-4 md:flex-row md:items-baseline md:gap-8">
          <p className="font-medium">{services.industries.label}</p>
          <ul className="flex flex-wrap gap-x-6 gap-y-2 text-fg-muted">
            {services.industries.items.map((name) => (
              <li key={name}>{name}</li>
            ))}
          </ul>
          <TextLink href={services.industries.link.href} tone="accent" className="md:ml-auto">
            {services.industries.link.label} →
          </TextLink>
        </div>
      </Container>
    </section>
  );
}
