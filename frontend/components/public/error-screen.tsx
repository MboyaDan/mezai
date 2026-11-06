import type { ReactNode } from "react";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { Container } from "./layout";

/**
 * 404 and 500 pages: the lattice left scattered, with no agreement dot,
 * because nothing lined up. Static, so reduced motion changes nothing.
 */
export function ErrorScreen({
  code,
  title,
  text,
  children,
  reference,
}: {
  code: string;
  title: string;
  text: string;
  children?: ReactNode;
  /** Shown so a visitor can quote it to us; never an error message. */
  reference?: string;
}) {
  return (
    <section aria-labelledby="error-title" className="bg-bg">
      <Container className="grid gap-12 py-20 md:grid-cols-12 md:items-center md:py-32">
        <div className="md:col-span-7">
          <p className="font-mono text-xs tracking-wide text-fg-muted">Error {code}</p>
          <h1
            id="error-title"
            className="mt-5 font-display text-[2.5rem] leading-[1.04] font-medium tracking-[-0.02em] md:text-display-2"
          >
            {title}
          </h1>
          <p className="mt-6 max-w-[34rem] text-lg text-fg-muted">{text}</p>
          {children && <div className="mt-10 flex flex-wrap gap-3">{children}</div>}
          {reference && (
            <p className="mt-8 font-mono text-xs text-fg-muted">
              Reference: <span className="select-all">{reference}</span>
            </p>
          )}
        </div>
        <div className="mx-auto w-48 md:col-span-4 md:col-start-9 md:w-full md:max-w-72">
          <SignalLattice rows={5} cols={5} state="scattered" accent={null} seed={404} />
        </div>
      </Container>
    </section>
  );
}
