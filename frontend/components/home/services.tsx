import { Plus } from "lucide-react";
import { TextLink } from "@/components/ui/text-link";
import { Bracket } from "@/components/brand/bracket";
import { services } from "@/content/home";
import { Accent, Container, Eyebrow, SectionTitle } from "./layout";
import { serviceDemo } from "./service-demos";

/** Frame 3. Rows on desktop; an accordion on mobile (DESIGN.md section 10). */
export function Services() {
  return (
    <section id="services" aria-labelledby="services-title" className="bg-bg">
      <Container className="py-20 md:py-32">
        <div className="grid gap-4 md:grid-cols-12">
          <Eyebrow className="md:col-span-3 md:pt-4">{services.eyebrow}</Eyebrow>
          <SectionTitle id="services-title" className="md:col-span-8 md:col-start-4">
            {services.title.lead} <Accent>{services.title.accent}</Accent> {services.title.tail}
          </SectionTitle>
        </div>

        {/* Desktop: all rows open, the first framed by the bracket. */}
        <ol className="mt-16 hidden border-t border-border-strong md:block">
          {services.items.map((s, i) => {
            const Demo = serviceDemo[s.key];
            const row = (
              <div className="grid grid-cols-12 items-start gap-6 py-8">
                <span className="col-span-1 font-mono text-xs text-fg-muted">0{i + 1}</span>
                <h3 className="col-span-4 font-display text-2xl font-medium">{s.name}</h3>
                <p className="col-span-3 text-fg-muted">{s.line}</p>
                <div className="col-span-4">
                  <Demo />
                </div>
              </div>
            );
            return (
              <li key={s.key} id={`service-${s.key}`} className="scroll-mt-24 border-b border-border">
                {i === 0 ? <Bracket tone="fg">{row}</Bracket> : row}
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
