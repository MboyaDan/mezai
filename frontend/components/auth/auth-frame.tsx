import Link from "next/link";
import type { ReactNode } from "react";
import { Logo } from "@/components/brand/logo";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { Accent } from "@/components/public/layout";
import { brand } from "@/lib/brand";

/**
 * Sign-in and sign-up frame: a charcoal panel with the lattice settling into
 * line (the brand idea: many judgments, one verified answer), and the form on
 * ivory. Under 1024 px the panel shrinks to a top bar.
 */
export function AuthFrame({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-dvh bg-bg lg:grid lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
      <aside data-theme="dark" className="relative overflow-hidden bg-bg text-fg">
        <div className="flex h-14 items-center px-4 sm:px-8 lg:h-auto lg:px-12 lg:pt-10">
          <Link href="/" aria-label={`${brand.name} home`}>
            <Logo title="" className="h-6" />
          </Link>
        </div>
        <div className="hidden px-12 pt-24 pb-12 lg:block">
          <p className="max-w-sm font-display text-display-3 leading-tight font-medium tracking-[-0.02em]">
            Careful judgments, <Accent>verified</Accent>.
          </p>
          <p className="mt-4 max-w-sm text-fg-muted">
            {brand.name} is {brand.descriptor}.
          </p>
          <div className="mt-16 w-64" aria-hidden="true">
            <SignalLattice rows={6} cols={9} state="align-on-load" />
          </div>
        </div>
      </aside>
      <main id="main" className="flex justify-center px-4 py-10 sm:px-8 lg:items-center lg:py-16">
        <div className="w-full max-w-md">{children}</div>
      </main>
    </div>
  );
}

export function AuthHeading({ title, children }: { title: ReactNode; children?: ReactNode }) {
  return (
    <div className="mb-8 grid gap-2">
      <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em]">{title}</h1>
      {children && <div className="text-fg-muted">{children}</div>}
    </div>
  );
}
