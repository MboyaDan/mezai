import { ButtonLink } from "@/components/ui/button";
import { TextLink } from "@/components/ui/text-link";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { hero, services } from "@/content/home";
import { Accent, Container } from "./layout";

/** Frame 1. Reduced-motion final state; Stage 5b adds the line reveals. */
export function Hero() {
  return (
    <section data-theme="dark" aria-labelledby="hero-title" className="relative overflow-hidden bg-bg">
      {/* The one gradient allowed: a faint glow behind the lattice. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -top-24 right-[-10%] size-[40rem] rounded-full bg-[radial-gradient(closest-side,rgb(200_240_49/0.07),transparent)] max-md:hidden"
      />
      <Container className="relative grid gap-12 pt-14 pb-16 md:grid-cols-12 md:items-center md:pt-28 md:pb-24">
        <div className="md:col-span-7">
          <p className="flex items-center gap-2 font-mono text-xs tracking-wide text-fg-muted">
            <span aria-hidden="true" className="size-1.5 rounded-full bg-lime" />
            {hero.eyebrow}
          </p>
          <h1
            id="hero-title"
            className="mt-6 font-display text-[2.75rem] leading-[1.02] font-medium tracking-[-0.02em] sm:text-[3.5rem] lg:text-display-1"
          >
            <span className="hero-line">
              <span className="hero-line-inner">
                {hero.headline.lead}{" "}
                <span className="hero-accent">
                  <Accent>{hero.headline.accent}</Accent>
                </span>
              </span>
            </span>
            <span className="hero-line mt-1">
              <span
                className="hero-line-inner text-fg-muted"
                style={{ "--delay": "80ms" } as React.CSSProperties}
              >
                {hero.headline.second}
              </span>
            </span>
          </h1>
          <p
            className="hero-after mt-6 max-w-[36rem] text-lg text-fg-muted md:text-xl"
            style={{ "--delay": "300ms" } as React.CSSProperties}
          >
            {hero.subline}
          </p>
          <div className="hero-after mt-9 flex flex-col gap-4 sm:flex-row sm:items-center sm:gap-6">
            <ButtonLink href={hero.primary.href} size="lg" arrow className="max-sm:w-full">
              {hero.primary.label}
            </ButtonLink>
            <TextLink href={hero.secondary.href} className="max-sm:self-center">
              {hero.secondary.label}
            </TextLink>
          </div>
        </div>
        <figure className="w-full md:col-span-5 md:col-start-8 lg:col-span-4 lg:col-start-9">
          {/* Mobile: a short band under the headline (DESIGN.md section 10). */}
          <SignalLattice state="align-on-load" rows={3} cols={11} accent={[1, 5]} className="md:hidden" />
          <SignalLattice state="align-on-load" className="hidden md:block" />
          <figcaption className="mt-4 flex justify-between font-mono text-[0.6875rem] text-fg-muted">
            <span>{hero.latticeCaption[0]}</span>
            <span>{hero.latticeCaption[1]}</span>
          </figcaption>
        </figure>
      </Container>
      <nav aria-label="Services" className="border-t border-border">
        <Container>
          <ol className="flex gap-6 overflow-x-auto py-4 font-mono text-xs text-fg-muted max-md:hidden">
            {services.items.map((s, i) => (
              <li key={s.key} className="shrink-0">
                <a href={`#service-${s.key}`} className="hover:text-fg">
                  <span className={i === 0 ? "text-fg" : undefined}>0{i + 1}</span> {s.name}
                </a>
              </li>
            ))}
          </ol>
        </Container>
      </nav>
    </section>
  );
}
