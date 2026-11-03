import { TextLink } from "@/components/ui/text-link";
import { cn } from "@/lib/cn";
import { quality } from "@/content/home";
import { Accent, Container, Eyebrow, SectionTitle } from "./layout";
import { RoutingCalculator } from "./calculator";

/** Frame 6. Static diagram; Stage 5b draws the connecting lines on scroll. */
export function Quality() {
  return (
    <section id="quality" aria-labelledby="quality-title" className="bg-bg">
      <Container className="py-20 md:py-32">
        <Eyebrow>{quality.eyebrow}</Eyebrow>
        <SectionTitle id="quality-title" className="mt-4">
          {quality.title.lead} <Accent>{quality.title.accent}</Accent>
        </SectionTitle>

        <ol className="mt-14 grid gap-0 lg:grid-cols-6">
          {quality.steps.map((s, i) => (
            <li
              key={s.name}
              className="relative grid grid-cols-[2rem_1fr] gap-x-4 pb-8 lg:block lg:pr-6 lg:pb-0"
            >
              {/* Connector: vertical on mobile, horizontal on desktop. */}
              <span aria-hidden="true" className="relative flex justify-center lg:block">
                <span
                  className={cn(
                    "relative z-10 grid size-8 place-items-center rounded-full border font-mono text-xs",
                    i === quality.steps.length - 1
                      ? "border-charcoal-950 bg-lime text-charcoal-950"
                      : "border-border-strong bg-bg",
                  )}
                >
                  {i + 1}
                </span>
                {i < quality.steps.length - 1 && (
                  <span className="absolute top-8 bottom-[-2rem] left-1/2 w-px bg-border-strong lg:top-4 lg:right-0 lg:bottom-auto lg:left-8 lg:h-px lg:w-auto" />
                )}
              </span>
              <div className="lg:mt-5">
                <h3 className="font-display text-lg font-medium">{s.name}</h3>
                <p className="mt-1 text-sm text-fg-muted">{s.line}</p>
              </div>
            </li>
          ))}
        </ol>

        <div className="mt-16 grid gap-10 lg:grid-cols-12">
          <div className="grid content-start gap-6 lg:col-span-5">
            <dl className="grid gap-6">
              {quality.definitions.map((d) => (
                <div key={d.term} className="border-t border-border pt-4">
                  <dt className="font-display text-lg font-medium">{d.term}</dt>
                  <dd className="mt-1 text-fg-muted">{d.text}</dd>
                </div>
              ))}
            </dl>
            <TextLink href={quality.link.href}>{quality.link.label} →</TextLink>
          </div>
          <div className="lg:col-span-6 lg:col-start-7">
            <RoutingCalculator />
          </div>
        </div>
      </Container>
    </section>
  );
}
