"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { ButtonLink } from "@/components/ui/button";
import { MotionProvider } from "@/motion/provider";
import { Reveal, RevealGroup, RevealItem } from "@/motion/reveal";
import { useProfile } from "./portal";

// The funnel after the profile (D-100), in the order currentStep() walks it.
const next = [
  {
    title: "Screening",
    body: "A person on the MezAI team reads your application and decides whether to shortlist you.",
  },
  {
    title: "Core check",
    body: "Once you are shortlisted, a short general test opens on your dashboard.",
  },
  {
    title: "Phone interview",
    body: "A short call with fixed questions. It also confirms your phone number.",
  },
  {
    title: "Identity check",
    body: "We check your National ID number and M-Pesa details before you start paid work.",
  },
];

/**
 * /c/profile/complete: shown once, when a profile is completed for the first
 * time. The lattice aligns into the lime node (DESIGN.md motion pattern 3);
 * reduced motion shows it already aligned. Nothing here promises a date.
 */
export function ApplicationReceived() {
  const { profile } = useProfile();
  const router = useRouter();
  const applied = profile.status === "applied";
  useEffect(() => {
    if (applied) router.replace("/c/profile");
  }, [applied, router]);
  if (applied) return null;

  return (
    // Motion loads only where it is used (D-050); this screen is one of those places.
    <MotionProvider>
      <div className="grid items-center gap-10 py-4 md:grid-cols-12 md:py-10">
        <div aria-hidden="true" className="mx-auto w-40 md:col-span-4 md:w-full md:max-w-64">
          <SignalLattice rows={7} cols={7} state="align-on-load" seed={45} />
        </div>
        <div className="grid gap-8 md:col-span-8">
          <Reveal className="grid gap-3">
            <p className="font-mono text-xs tracking-wide text-fg-muted uppercase">Profile complete</p>
            <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
              Application received
            </h1>
            <p className="max-w-prose text-fg-muted">
              Thank you. Your profile is saved and your application is in the queue. Your contributor ID is{" "}
              <span className="font-mono text-fg">{profile.public_code}</span>.
            </p>
          </Reveal>
          <section aria-labelledby="next-h" className="grid gap-4">
            <h2 id="next-h" className="font-display text-lg font-semibold">
              What happens next
            </h2>
            <RevealGroup as="ul" className="grid gap-3">
              {next.map((s, i) => (
                <RevealItem
                  as="li"
                  key={s.title}
                  className="flex gap-4 rounded-card border border-border p-4"
                >
                  <span
                    aria-hidden="true"
                    className="grid size-7 shrink-0 place-items-center rounded-full border border-border-strong/60 font-mono text-xs"
                  >
                    {i + 1}
                  </span>
                  <span className="grid gap-0.5">
                    <span className="font-medium">{s.title}</span>
                    <span className="text-sm text-fg-muted">{s.body}</span>
                  </span>
                </RevealItem>
              ))}
            </RevealGroup>
            <p className="text-sm text-fg-muted">Your dashboard always shows the step you are on.</p>
          </section>
          <div className="flex flex-wrap gap-3">
            <ButtonLink href="/c" arrow>
              Go to your dashboard
            </ButtonLink>
            <ButtonLink href="/c/profile" variant="secondary">
              Review your profile
            </ButtonLink>
          </div>
        </div>
      </div>
    </MotionProvider>
  );
}
