import { Check } from "lucide-react";
import type { ServiceKey } from "@/content/home";

// Tiny product illustrations for frame 3. Fictional data, labelled
// "Demonstration". Stage 5b plays each one on hover or open.

function Frame({ kind, children }: { kind: string; children: React.ReactNode }) {
  return (
    <div className="rounded-card border border-border bg-surface p-4 text-sm">
      <div className="mb-3 flex justify-between font-mono text-[0.625rem] tracking-wider text-fg-muted uppercase">
        <span>{kind}</span>
        <span>Demonstration</span>
      </div>
      {children}
    </div>
  );
}

function Pairwise() {
  return (
    <Frame kind="Pairwise">
      <div className="grid grid-cols-2 gap-2">
        <div className="rounded-control border border-border-strong p-2.5">
          <p className="font-mono text-[0.625rem] text-fg-muted">A</p>
          <p className="mt-1 line-clamp-2">Water seedlings lightly each morning; check the soil first.</p>
        </div>
        <div className="rounded-control border border-border p-2.5">
          <p className="font-mono text-[0.625rem] text-fg-muted">B</p>
          <p className="mt-1 line-clamp-2">Flood the tray twice a day.</p>
        </div>
      </div>
      <p className="mt-3 flex items-center gap-1.5 text-xs font-medium text-accent-fg">
        <span aria-hidden="true" className="size-2 rounded-full bg-accent-fg" />A preferred · accuracy 5 ·
        language 4
      </p>
    </Frame>
  );
}

function Translation() {
  return (
    <Frame kind="Translation review">
      <p>
        <span className="mr-2 font-mono text-[0.625rem] text-fg-muted">SW</span>Soko linafunguliwa saa mbili
        asubuhi.
      </p>
      <p className="mt-1.5">
        <span className="mr-2 font-mono text-[0.625rem] text-fg-muted">EN</span>The market opens at{" "}
        <span className="underline decoration-laterite decoration-2 underline-offset-2">
          eight in the morning
        </span>
        .
      </p>
      <p className="mt-3 font-mono text-[0.6875rem] text-fg-muted">adequacy 5 · fluency 5</p>
    </Frame>
  );
}

function Classification() {
  return (
    <Frame kind="Classification">
      <p>&ldquo;Nimeongeza salio lakini bado halijaonekana.&rdquo;</p>
      <ul className="mt-3 flex flex-wrap gap-1.5 text-xs" aria-label="Labels">
        {["Billing", "Network", "Other"].map((l, i) => (
          <li
            key={l}
            className={
              i === 0
                ? "rounded-full border border-border-strong bg-inverse-bg px-2.5 py-0.5 text-inverse-fg"
                : "rounded-full border border-border px-2.5 py-0.5"
            }
          >
            <span className="mr-1 font-mono text-[0.625rem] opacity-70">{i + 1}</span>
            {l}
          </li>
        ))}
      </ul>
    </Frame>
  );
}

function Review() {
  return (
    <Frame kind="Review">
      <div className="flex items-center gap-3">
        <span className="grid size-8 shrink-0 place-items-center rounded-full border border-success text-success">
          <Check aria-hidden="true" className="size-4" />
        </span>
        <div>
          <p className="font-mono text-xs">ITEM 0412 · 3 of 3 agree</p>
          <p className="text-xs text-fg-muted">Accepted by reviewer R-07</p>
        </div>
      </div>
    </Frame>
  );
}

export const serviceDemo: Record<ServiceKey, () => React.ReactElement> = {
  evaluation: Pairwise,
  language: Translation,
  annotation: Classification,
  review: Review,
};
