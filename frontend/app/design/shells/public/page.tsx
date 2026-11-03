import { PublicShell } from "@/components/shells/public-shell";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { ButtonLink } from "@/components/ui/button";

export default function PublicShellPreview() {
  return (
    <PublicShell>
      <section data-theme="dark" className="bg-bg">
        <div className="mx-auto grid max-w-[82rem] gap-10 px-4 py-16 sm:px-8 md:grid-cols-12 md:items-center lg:px-16">
          <div className="md:col-span-7">
            <h1 className="font-display text-[2.75rem] leading-[1.04] font-medium tracking-[-0.02em] md:text-display-1">
              Shell <span className="font-serif">preview.</span>
            </h1>
            <p className="mt-5 max-w-xl text-lg text-fg-muted">Public pages alternate dark and ivory bands.</p>
            <div className="mt-8 flex flex-col gap-3 sm:flex-row">
              <ButtonLink href="/design" arrow>
                Primary action
              </ButtonLink>
              <ButtonLink href="/design" variant="secondary">
                Secondary
              </ButtonLink>
            </div>
          </div>
          <div className="max-w-xs md:col-span-4 md:col-start-9 md:max-w-none">
            <SignalLattice />
          </div>
        </div>
      </section>
      <section className="mx-auto max-w-[82rem] px-4 py-16 sm:px-8 lg:px-16">
        <h2 className="font-display text-[1.75rem] font-medium md:text-display-3">An ivory band</h2>
        <p className="mt-4 max-w-[68ch] text-fg-muted">Reading content sits on ivory with ink text at 16 px.</p>
      </section>
    </PublicShell>
  );
}
