"use client";

import { useState } from "react";
import { quality } from "@/content/home";
import { routeItem } from "@/lib/routing";
import { cn } from "@/lib/cn";

const people = ["Person 1", "Person 2", "Person 3"];

/** Frame 6 calculator: pick three labels, see whether the item routes to review. */
export function RoutingCalculator() {
  const { labels } = quality.calculator;
  const [picks, setPicks] = useState<string[]>([labels[0], labels[0], labels[1]]);
  const result = routeItem(picks);
  return (
    <div className="rounded-card border border-border bg-surface p-5 md:p-6">
      <h3 className="font-display text-lg font-medium">{quality.calculator.title}</h3>
      <p className="mt-1 text-sm text-fg-muted">{quality.calculator.intro}</p>
      <div className="mt-5 grid gap-4">
        {people.map((p, i) => (
          <fieldset key={p} className="grid gap-2 sm:grid-cols-[6rem_1fr] sm:items-center">
            <legend className="float-left font-mono text-xs text-fg-muted sm:pt-2">{p}</legend>
            <div className="flex flex-wrap gap-2 sm:col-start-2">
              {labels.map((l) => {
                const on = picks[i] === l;
                return (
                  <label
                    key={l}
                    className={cn(
                      "cursor-pointer rounded-full border px-3 py-1 text-sm transition-colors duration-(--dur-quick) has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-focus",
                      on
                        ? "border-border-strong bg-inverse-bg text-inverse-fg"
                        : "border-border hover:border-border-strong",
                    )}
                  >
                    <input
                      type="radio"
                      name={`person-${i}`}
                      value={l}
                      checked={on}
                      onChange={() => setPicks((prev) => prev.map((v, j) => (j === i ? l : v)))}
                      className="sr-only"
                    />
                    {l}
                  </label>
                );
              })}
            </div>
          </fieldset>
        ))}
      </div>
      <p
        key={picks.join("|")}
        aria-live="polite"
        className={cn(
          "swap-in mt-6 border-t border-border pt-4 font-medium",
          result.kind === "review" ? "text-fg" : "text-accent-fg",
        )}
      >
        {result.kind === "consensus"
          ? `${result.agreeing} of ${result.total} agree: “${result.label}” becomes the consensus answer.`
          : `No majority: the item goes to a reviewer, who writes the final answer.`}
      </p>
    </div>
  );
}
