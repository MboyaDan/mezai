import type { ReactNode } from "react";
import { Accent, Container } from "./layout";

export type Title = { lead: string; accent?: string; tail?: string };

/** The title as text, for metadata. */
export const titleText = (t: Title) => [t.lead, t.accent, t.tail].filter(Boolean).join(" ");

/**
 * Opening band for inner public pages: charcoal like the home hero, but
 * shorter, with the title on seven columns and an optional aside.
 */
export function PageHeader({
  eyebrow,
  title,
  intro,
  children,
  aside,
}: {
  eyebrow: string;
  title: Title;
  intro: string;
  children?: ReactNode;
  aside?: ReactNode;
}) {
  return (
    <section data-theme="dark" aria-labelledby="page-title" className="bg-bg">
      <Container className="grid gap-10 pt-12 pb-14 md:grid-cols-12 md:items-end md:pt-20 md:pb-20">
        <div className="md:col-span-8 lg:col-span-7">
          <p className="flex items-center gap-2 font-mono text-xs tracking-wide text-fg-muted">
            <span aria-hidden="true" className="size-1.5 rounded-full bg-lime" />
            {eyebrow}
          </p>
          <h1
            id="page-title"
            className="mt-5 font-display text-[2.5rem] leading-[1.04] font-medium tracking-[-0.02em] sm:text-[3rem] lg:text-display-2"
          >
            {title.lead}
            {title.accent && (
              <>
                {" "}
                <Accent>{title.accent}</Accent>
              </>
            )}
            {title.tail && <> {title.tail}</>}
          </h1>
          <p className="mt-6 max-w-[38rem] text-lg text-fg-muted">{intro}</p>
          {children && <div className="mt-8">{children}</div>}
        </div>
        {aside && <div className="md:col-span-4 md:col-start-9 lg:col-span-4 lg:col-start-9">{aside}</div>}
      </Container>
    </section>
  );
}
