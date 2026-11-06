"use client";

import { useId, useState } from "react";
import { Agreement } from "@/components/ui/data";
import { cohensKappa, twoRaterExample } from "@/lib/kappa";
import { cn } from "@/lib/cn";

// Small live diagrams for /quality (brief 6.1: "explained with live
// diagrams"). Every value is a worked example, never a project result.

function Panel({
  title,
  children,
  className,
}: {
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <figure data-demo className={cn("rounded-card border border-border bg-surface p-5 md:p-6", className)}>
      <figcaption className="mb-4 flex items-baseline justify-between gap-4 font-mono text-[0.6875rem] tracking-wider text-fg-muted uppercase">
        <span>{title}</span>
        <span>Worked example</span>
      </figcaption>
      {children}
    </figure>
  );
}

/** Gold mix: one contributor's queue of 20 items; reveal which are gold. */
export function GoldMix() {
  const [reveal, setReveal] = useState(false);
  const gold = new Set([6, 15]); // 2 of 20: the 10 percent default
  return (
    <Panel title="One contributor's queue">
      <ol className="grid grid-cols-10 gap-1.5" aria-label="20 items in a queue">
        {Array.from({ length: 20 }, (_, i) => {
          const isGold = reveal && gold.has(i);
          return (
            <li
              key={i}
              className={cn(
                "grid aspect-square place-items-center rounded-[2px] border font-mono text-[0.625rem] transition-colors duration-(--dur-base)",
                isGold ? "border-series-3 bg-series-3/25 text-fg" : "border-border-strong text-fg-muted",
              )}
            >
              {i + 1}
              {isGold && <span className="sr-only"> (gold)</span>}
            </li>
          );
        })}
      </ol>
      <div className="mt-5 flex flex-wrap items-center justify-between gap-3">
        <p aria-live="polite" className="text-sm text-fg-muted">
          {reveal
            ? "Items 7 and 16 were gold: 2 of 20, the default rate."
            : "This is what the contributor sees: 20 items, all alike."}
        </p>
        <button
          type="button"
          aria-pressed={reveal}
          onClick={() => setReveal((v) => !v)}
          className="min-h-11 rounded-control border border-border-strong px-3 text-sm font-medium hover:bg-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
        >
          {reveal ? "Hide gold items" : "Show gold items"}
        </button>
      </div>
    </Panel>
  );
}

/** Overlap: choose k and see one item fan out to k different people. */
export function OverlapDiagram() {
  const [k, setK] = useState(3);
  const id = useId();
  const w = 320;
  const h = 150;
  const xs = Array.from({ length: k }, (_, i) => (w / (k + 1)) * (i + 1));
  return (
    <Panel title="Overlap">
      <fieldset>
        <legend className="text-sm font-medium">Contributors per item (k)</legend>
        <div className="mt-2 flex gap-2">
          {[1, 2, 3, 4, 5].map((n) => (
            <label
              key={n}
              className={cn(
                "grid size-11 cursor-pointer place-items-center rounded-control border font-mono text-sm transition-colors duration-(--dur-quick) has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-focus",
                n === k
                  ? "border-border-strong bg-inverse-bg text-inverse-fg"
                  : "border-border hover:border-border-strong",
              )}
            >
              <input
                type="radio"
                name={`${id}-k`}
                value={n}
                checked={n === k}
                onChange={() => setK(n)}
                className="sr-only"
              />
              {n}
            </label>
          ))}
        </div>
      </fieldset>
      <svg
        viewBox={`0 0 ${w} ${h}`}
        className="mt-5 w-full"
        role="img"
        aria-label={`One item sent to ${k} different contributors`}
      >
        <rect x={w / 2 - 34} y={4} width={68} height={28} rx={2} className="fill-bg stroke-fg" />
        <text x={w / 2} y={22} textAnchor="middle" className="fill-fg font-mono text-[11px]">
          item
        </text>
        {xs.map((x) => (
          <g key={x}>
            <path
              d={`M${w / 2} 32 C ${w / 2} 80, ${x} 70, ${x} 112`}
              className="fill-none stroke-border-strong"
            />
            <circle cx={x} cy={124} r={11} className="fill-bg stroke-fg" />
          </g>
        ))}
      </svg>
      <p aria-live="polite" className="mt-2 text-sm text-fg-muted">
        {k === 1
          ? "One answer per item: fine for low-risk work, with more gold items to compensate."
          : `${k} different people answer this item without seeing each other's answers.`}
      </p>
    </Panel>
  );
}

/** Kappa explorer: percent agreement and Cohen's kappa for two raters. */
export function KappaExplorer() {
  const total = 50;
  const [agree, setAgree] = useState(42);
  const [share, setShare] = useState(50);
  const ids = useId();
  const m = twoRaterExample(total, agree, share / 100);
  const r = cohensKappa(m);
  const cells: Array<{ label: string; v: number; same: boolean }> = [
    { label: "Both: spam", v: m[0][0], same: true },
    { label: "A spam, B not", v: m[0][1], same: false },
    { label: "A not, B spam", v: m[1][0], same: false },
    { label: "Both: not spam", v: m[1][1], same: true },
  ];
  return (
    <Panel title="Two raters, 50 messages">
      <div className="grid gap-5">
        <div className="grid gap-2">
          <label htmlFor={`${ids}-agree`} className="flex justify-between text-sm font-medium">
            <span>Messages they label the same</span>
            <span className="font-mono tabular">
              {agree} of {total}
            </span>
          </label>
          <input
            id={`${ids}-agree`}
            type="range"
            min={25}
            max={50}
            value={agree}
            onChange={(e) => setAgree(Number(e.target.value))}
            className="accent-accent-fg"
          />
        </div>
        <div className="grid gap-2">
          <label htmlFor={`${ids}-share`} className="flex justify-between text-sm font-medium">
            <span>Of those, labelled spam</span>
            <span className="font-mono tabular">{share}%</span>
          </label>
          <input
            id={`${ids}-share`}
            type="range"
            min={50}
            max={98}
            step={2}
            value={share}
            onChange={(e) => setShare(Number(e.target.value))}
            className="accent-accent-fg"
          />
        </div>
      </div>
      <dl className="mt-5 grid grid-cols-2 gap-px overflow-hidden rounded-control border border-border bg-border">
        {cells.map((c) => (
          <div key={c.label} className={cn("p-3", c.same ? "bg-bg" : "bg-sunken")}>
            <dt className="text-xs text-fg-muted">{c.label}</dt>
            <dd className="font-mono text-lg tabular">{c.v}</dd>
          </div>
        ))}
      </dl>
      <div aria-live="polite" className="mt-5 grid grid-cols-2 gap-4 border-t border-border pt-4">
        <div>
          <p className="text-xs text-fg-muted">Percent agreement</p>
          <p className="font-mono text-xl tabular sm:text-2xl">{r ? `${Math.round(r.po * 100)}%` : "–"}</p>
        </div>
        <div>
          <p className="text-xs text-fg-muted">Cohen&apos;s kappa</p>
          <p className="text-xl sm:text-2xl">
            {r ? <Agreement value={r.kappa} /> : <span className="font-mono">undefined</span>}
          </p>
        </div>
      </div>
      <p className="mt-3 text-sm text-fg-muted">
        {r && r.po - r.kappa > 0.35
          ? "When nearly everything is spam, agreeing is easy by chance alone, so kappa stays low."
          : "Kappa removes the agreement you would expect by chance."}
      </p>
    </Panel>
  );
}
