"use client";

import { useEffect, useRef, useState } from "react";
import { ArrowLeft, ArrowRight, Check, Pause, Play } from "lucide-react";
import { Button } from "@/components/ui/button";
import { ProgressBar, Agreement } from "@/components/ui/data";
import { StatusPill } from "@/components/ui/display";
import { Bracket } from "@/components/brand/bracket";
import { demo } from "@/content/home";
import { cn } from "@/lib/cn";
import { useMediaQuery } from "@/lib/use-media-query";

// The product demonstration (DESIGN.md section 10, frame 5). On screens 768 px
// and wider it plays by itself while in view and pauses when out of view;
// any manual step change stops it. Reduced motion and phones get the manual
// step-by-step version only. All data is fictional.

const STEP_MS = 4500;

const source = "Ofisi yetu imefungwa siku za sikukuu.";

function Scores({ adequacy, fluency }: { adequacy: number; fluency: number }) {
  return (
    <dl className="grid grid-cols-2 gap-3 text-sm">
      {[
        ["Adequacy", adequacy],
        ["Fluency", fluency],
      ].map(([label, v]) => (
        <div key={label as string}>
          <dt className="text-fg-muted">{label}</dt>
          <dd className="mt-1 flex gap-1" aria-label={`${v} of 5`}>
            {[1, 2, 3, 4, 5].map((n) => (
              <span
                key={n}
                className={cn(
                  "grid size-7 place-items-center rounded-control border font-mono text-xs",
                  n === v
                    ? "border-border-strong bg-inverse-bg text-inverse-fg"
                    : "border-border text-fg-muted",
                )}
              >
                {n}
              </span>
            ))}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function Panel({ step }: { step: number }) {
  if (step <= 1) {
    return (
      <div className="grid gap-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="rounded-control border border-border p-3">
            <p className="font-mono text-[0.625rem] text-fg-muted">SOURCE · EN</p>
            <p className="mt-1">Our office is closed on public holidays.</p>
          </div>
          <div className="rounded-control border border-border p-3">
            <p className="font-mono text-[0.625rem] text-fg-muted">TRANSLATION · SW</p>
            <p className="mt-1">
              {step === 0 ? (
                source
              ) : (
                <>
                  Ofisi yetu{" "}
                  <span className="underline decoration-laterite decoration-2 underline-offset-2">
                    hufungwa
                  </span>{" "}
                  siku za sikukuu.
                </>
              )}
            </p>
          </div>
        </div>
        <Scores adequacy={4} fluency={3} />
        <div className="flex items-center justify-between border-t border-border pt-3 text-sm">
          <span className="font-mono text-xs text-fg-muted">Item 12 of 40</span>
          {step === 1 ? (
            <StatusPill status="success">Submitted</StatusPill>
          ) : (
            <span className="text-fg-muted">Draft saved</span>
          )}
        </div>
      </div>
    );
  }
  if (step === 2) {
    return (
      <div className="grid gap-3">
        <p className="font-mono text-xs text-fg-muted">ITEM 0412 · QUALITY REVIEW</p>
        <ul className="grid gap-2 text-sm">
          {[
            ["C-0014", "4 / 3"],
            ["C-0009", "4 / 4"],
            ["C-0021", "5 / 4"],
          ].map(([id, s]) => (
            <li key={id} className="flex justify-between rounded-control border border-border px-3 py-2">
              <span className="font-mono">{id}</span>
              <span className="text-fg-muted">adequacy / fluency {s}</span>
            </li>
          ))}
        </ul>
        <p className="flex items-center gap-2 text-sm">
          <Check aria-hidden="true" className="size-4 text-accent-fg" /> Spread within threshold. Reviewer
          R-07 accepts the consensus.
        </p>
      </div>
    );
  }
  if (step === 3) {
    return (
      <div className="grid gap-4">
        <p className="font-mono text-xs text-fg-muted">PROJECT AGREEMENT</p>
        <div className="flex flex-wrap items-baseline gap-x-6 gap-y-2">
          <Agreement value={0.68} statistic="κ" className="text-3xl" />
          <span className="text-sm text-fg-muted">was 0.67 · percent agreement 84%</span>
        </div>
        <ProgressBar label="Items with consensus" value={318} max={400} valueText="318 of 400" />
      </div>
    );
  }
  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between">
        <p className="font-medium">Swahili support replies (Sample)</p>
        <StatusPill status="info">Live</StatusPill>
      </div>
      <ProgressBar label="Completed" value={1240} max={2000} valueText="1,240 of 2,000" />
      <dl className="grid grid-cols-3 gap-4 border-t border-border pt-4">
        {[
          ["Gold accuracy", "0.91"],
          ["Agreement", "0.68"],
          ["Review rate", "7%"],
        ].map(([k, v]) => (
          <div key={k}>
            <dt className="text-xs text-fg-muted">{k}</dt>
            <dd className="mt-1 font-mono text-xl tabular">{v}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

export function DemoWindow() {
  const [step, setStep] = useState(0);
  const canPlay = useMediaQuery("(min-width: 768px) and (prefers-reduced-motion: no-preference)");
  const [stopped, setStopped] = useState(false);
  const playing = canPlay && !stopped;
  const [inView, setInView] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const last = demo.steps.length - 1;

  useEffect(() => {
    const el = root.current;
    if (!el) return;
    const io = new IntersectionObserver(([e]) => setInView(e.isIntersecting), { threshold: 0.4 });
    io.observe(el);
    return () => io.disconnect();
  }, []);

  useEffect(() => {
    if (!playing || !inView) return;
    const t = window.setTimeout(
      () => setStep((s) => (s === last ? 0 : s + 1)),
      step === last ? STEP_MS * 1.5 : STEP_MS,
    );
    return () => window.clearTimeout(t);
  }, [playing, inView, step, last]);

  const go = (next: number) => {
    setStopped(true);
    setStep(next);
  };
  const view = step === last ? "Client view" : "Contributor and reviewer view";
  return (
    <Bracket tone="accent">
      <div ref={root} data-demo className="overflow-hidden rounded-card border border-border bg-surface shadow-float">
        <div className="flex items-center justify-between gap-4 border-b border-border px-4 py-3">
          <p className="font-mono text-xs text-fg-muted">{view}</p>
          <span className="rounded-full border border-border px-2.5 py-0.5 font-mono text-[0.625rem] tracking-wider text-fg uppercase">
            {demo.label}
          </span>
        </div>
        <div className="grid gap-6 p-5 md:grid-cols-[1fr_16rem] md:p-6">
          <div key={step} className="swap-in min-h-[15rem]">
            <Panel step={step} />
          </div>
          <div className="grid content-between gap-6 border-border md:border-l md:pl-6">
            <div aria-live="polite">
              <p className="font-mono text-xs text-fg-muted">
                Step {step + 1} of {demo.steps.length}
              </p>
              <p className="mt-2 font-display text-lg font-medium">{demo.steps[step].title}</p>
              <p className="mt-2 text-sm text-fg-muted">{demo.steps[step].caption}</p>
            </div>
            <div className="grid gap-4">
              <ol className="flex gap-1.5" aria-label="Steps">
                {demo.steps.map((s, i) => (
                  <li key={s.title}>
                    <button
                      type="button"
                      onClick={() => go(i)}
                      aria-label={`Step ${i + 1}: ${s.title}`}
                      aria-current={i === step ? "step" : undefined}
                      className="grid size-6 place-items-center"
                    >
                      <span
                        className={cn(
                          "block h-1 w-5 rounded-full",
                          i === step ? "bg-accent-fg" : "bg-border",
                        )}
                      />
                    </button>
                  </li>
                ))}
              </ol>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="secondary"
                  disabled={step === 0}
                  onClick={() => go(step - 1)}
                  icon={<ArrowLeft />}
                >
                  Back
                </Button>
                <Button size="sm" variant="secondary" disabled={step === last} onClick={() => go(step + 1)}>
                  Next <ArrowRight aria-hidden="true" className="size-4" />
                </Button>
                {canPlay && (
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-pressed={playing}
                    onClick={() => setStopped((v) => !v)}
                    icon={playing ? <Pause /> : <Play />}
                  >
                    {playing ? "Pause" : "Play"}
                  </Button>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </Bracket>
  );
}
