import { TextLink } from "@/components/ui/text-link";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { people } from "@/content/home";
import { Container, Eyebrow, SectionTitle } from "./layout";

/**
 * Frame 4. The storyboard's photo spread waits for credited photos in
 * docs/content (A-024); until then the frame is typographic and speaks of
 * who we look for, never of who "our contributors" are.
 */
export function People() {
  return (
    <section aria-labelledby="people-title" className="border-t border-border bg-bg">
      <Container className="grid gap-12 py-20 md:py-32 lg:grid-cols-12">
        <div className="lg:col-span-5">
          <Eyebrow>{people.eyebrow}</Eyebrow>
          <SectionTitle id="people-title" className="mt-4">
            {people.title}
          </SectionTitle>
          <p className="mt-6 max-w-md text-lg text-fg-muted">{people.intro}</p>
          <div className="mt-10 w-28">
            <SignalLattice rows={4} cols={6} accent={[2, 3]} />
          </div>
        </div>
        <div className="grid gap-12 lg:col-span-6 lg:col-start-7">
          <ul className="grid gap-px overflow-hidden rounded-card border border-border bg-border sm:grid-cols-2">
            {people.profiles.map((p) => (
              <li key={p.name} className="grid content-start gap-2 bg-surface p-6">
                <h3 className="font-display text-xl font-medium">{p.name}</h3>
                <p className="text-sm text-fg-muted">{p.line}</p>
              </li>
            ))}
          </ul>
          <div>
            <h3 className="font-mono text-xs tracking-wide text-fg-muted">How vetting works</h3>
            <ol className="mt-4 grid gap-4">
              {people.steps.map((step, i) => (
                <li key={i} className="grid grid-cols-[2rem_1fr] items-baseline border-t border-border pt-4">
                  <span className="font-mono text-sm text-fg-muted">0{i + 1}</span>
                  <span>{step}</span>
                </li>
              ))}
            </ol>
            <TextLink href={people.link.href} className="mt-6 inline-block">
              {people.link.label} →
            </TextLink>
          </div>
        </div>
      </Container>
    </section>
  );
}
