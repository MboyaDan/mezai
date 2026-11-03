import { ButtonLink } from "@/components/ui/button";
import { TextLink } from "@/components/ui/text-link";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { finalCta, practices } from "@/content/home";
import { Container } from "./layout";

/** Frame 7. Three practices we implement, each linked to its page. */
export function Practices() {
  return (
    <section data-theme="dark" aria-labelledby="practices-title" className="bg-bg">
      <Container className="py-16 md:py-20">
        <h2 id="practices-title" className="font-mono text-xs tracking-wide text-fg-muted">
          {practices.title}
        </h2>
        {/* A rule that draws across before the statements enter (pattern 7). */}
        <span aria-hidden="true" data-rule className="mt-8 block h-px origin-left bg-border max-md:hidden" />
        <ul
          className="mt-8 grid border-t border-border md:mt-6 md:grid-cols-3 md:border-t-0"
          data-reveal-group
        >
          {practices.items.map((p) => (
            <li
              key={p.text}
              className="grid content-between gap-4 border-b border-border py-6 md:border-b-0 md:border-l md:px-8 md:py-2 md:first:border-l-0 md:first:pl-0"
            >
              <p className="text-lg">{p.text}</p>
              <TextLink href={p.link.href} className="text-sm">
                {p.link.label} →
              </TextLink>
            </li>
          ))}
        </ul>
      </Container>
    </section>
  );
}

/** Frame 8. A calm lime band. */
export function FinalCta() {
  return (
    <section data-theme="light" aria-labelledby="cta-title" className="bg-lime text-charcoal-950">
      <Container className="flex flex-col gap-10 py-20 md:flex-row md:items-end md:justify-between md:py-28">
        <div data-reveal>
          <div className="w-20">
            <SignalLattice rows={3} cols={6} accent={null} />
          </div>
          <h2
            id="cta-title"
            className="mt-8 font-display text-[2.5rem] leading-[1.04] font-medium tracking-[-0.02em] md:text-display-2"
          >
            {finalCta.title}
          </h2>
        </div>
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:gap-6">
          <ButtonLink
            href={finalCta.primary.href}
            variant="inverse"
            size="lg"
            arrow
            className="max-sm:w-full"
          >
            {finalCta.primary.label}
          </ButtonLink>
          <TextLink href={finalCta.secondary.href} className="text-charcoal-950 max-sm:self-center">
            {finalCta.secondary.label}
          </TextLink>
        </div>
      </Container>
    </section>
  );
}
