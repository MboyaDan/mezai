import { ButtonLink } from "@/components/ui/button";
import { TextLink } from "@/components/ui/text-link";
import { Container } from "./layout";

type Action = { label: string; href: string };

/** Closing band for inner pages: charcoal, one action, quieter than the home lime band. */
export function CtaBand({
  title,
  text,
  primary,
  secondary,
}: {
  title: string;
  text?: string;
  primary: Action;
  secondary?: Action;
}) {
  return (
    <section data-theme="dark" aria-labelledby="cta-title" className="bg-bg">
      <Container className="py-16 md:py-24">
        <div aria-hidden="true" data-rule className="mb-10 h-px origin-left bg-border-strong" />
        <div className="grid gap-8 md:grid-cols-12 md:items-end">
          <div className="md:col-span-7">
            <h2
              id="cta-title"
              className="font-display text-[2rem] leading-[1.08] font-medium tracking-[-0.02em] md:text-display-3"
            >
              {title}
            </h2>
            {text && <p className="mt-4 max-w-xl text-lg text-fg-muted">{text}</p>}
          </div>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:gap-6 md:col-span-5 md:justify-end">
            <ButtonLink href={primary.href} size="lg" arrow className="max-sm:w-full">
              {primary.label}
            </ButtonLink>
            {secondary && (
              <TextLink href={secondary.href} className="max-sm:text-center">
                {secondary.label}
              </TextLink>
            )}
          </div>
        </div>
      </Container>
    </section>
  );
}
