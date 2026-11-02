import { ApiStatusBadge } from "@/components/api-status-badge";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { brand } from "@/lib/brand";

// Placeholder until the storyboard home page (Stage 5).
export default function Home() {
  return (
    <section data-theme="dark" className="bg-bg">
      <div className="mx-auto grid max-w-[82rem] gap-12 px-4 py-16 sm:px-8 md:grid-cols-12 md:items-center md:py-28 lg:px-16">
        <div className="md:col-span-7">
          <h1 className="font-display text-[2.75rem] leading-[1.04] font-medium tracking-[-0.02em] md:text-display-2">
            {brand.name}
          </h1>
          <p className="mt-5 max-w-xl text-lg text-fg-muted">
            {brand.name} is {brand.descriptor}, {brand.city}. The pilot platform is under construction.
          </p>
          <div className="mt-8">
            <ApiStatusBadge />
          </div>
        </div>
        <div className="max-w-xs md:col-span-4 md:col-start-9 md:max-w-none">
          <SignalLattice state="align-on-load" />
        </div>
      </div>
    </section>
  );
}
