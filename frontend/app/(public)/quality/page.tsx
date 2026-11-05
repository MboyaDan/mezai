import type { Metadata } from "next";
import type { ReactNode } from "react";
import { Container, Eyebrow } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { CtaBand } from "@/components/public/cta-band";
import { PageMotion } from "@/components/public/page-motion";
import { SignalLattice } from "@/components/brand/signal-lattice";
import { RoutingCalculator } from "@/components/home/calculator";
import { GoldMix, KappaExplorer, OverlapDiagram } from "@/components/quality/diagrams";
import { qualityPage } from "@/content/quality";
import { cn } from "@/lib/cn";

export const metadata: Metadata = {
  title: "How quality works",
  description: qualityPage.intro,
  alternates: { canonical: "/quality" },
  openGraph: { title: titleText(qualityPage.title) },
};

/** Fictional manifest for the "Deliver" step, labelled as a demonstration. */
function ManifestSample() {
  const { manifest } = qualityPage;
  const rows: Array<[string, string]> = [
    ["project", '"Sample intent labels (Sample)"'],
    ["exported_at", '"2026-01-15T09:30:00Z"'],
    ["schema_version", '"1"'],
    ["items", "{ total: 600, consensus: 541, adjudicated: 52, rejected: 7 }"],
    ["gold_accuracy", "{ project: 0.93, p10: 0.86, p90: 0.98 }"],
    ["agreement", '{ statistic: "fleiss_kappa", value: 0.71, percent: 0.84 }'],
    ["review_rate", "0.11"],
    ["contributors", '["C-0142", "C-0157", "C-0163", …]'],
    ["files", '[{ name: "labels.jsonl", sha256: "9f2c…e41a" }]'],
  ];
  return (
    <figure className="overflow-hidden rounded-card border border-border">
      <div
        data-theme="dark"
        className="flex items-center justify-between bg-bg px-4 py-2 font-mono text-[0.6875rem] tracking-wider text-fg-muted uppercase"
      >
        <span>manifest.json</span>
        <span className="text-accent-fg">{manifest.label}</span>
      </div>
      <pre
        tabIndex={0}
        aria-label="Sample manifest (fictional)"
        data-theme="dark"
        className="overflow-x-auto bg-surface p-4 font-mono text-xs leading-6 text-fg"
      >
        <code>
          {"{\n"}
          {rows.map(([k, v]) => (
            <span key={k}>
              {"  "}
              <span className="text-fg-muted">{k}</span>: {v},{"\n"}
            </span>
          ))}
          {"}"}
        </code>
      </pre>
      <figcaption className="border-t border-border bg-surface px-4 py-3 text-sm text-fg-muted">
        {manifest.caption}
      </figcaption>
    </figure>
  );
}

const visual: Record<string, ReactNode> = {
  gold: <GoldMix />,
  overlap: <OverlapDiagram />,
  agreement: <KappaExplorer />,
  review: <RoutingCalculator />,
  deliver: <ManifestSample />,
};

export default function QualityPage() {
  const { steps, bands, trust, manifest, security } = qualityPage;
  return (
    <>
      <PageHeader
        eyebrow={qualityPage.eyebrow}
        title={qualityPage.title}
        intro={qualityPage.intro}
        aside={
          <div className="mx-auto w-48 opacity-80 md:ml-auto md:w-full md:max-w-64" aria-hidden="true">
            <SignalLattice rows={6} cols={6} accent={[3, 3]} state="align-on-load" />
          </div>
        }
      />

      {/* The six steps: an index rail, then one long section per step. */}
      <section aria-labelledby="steps-title" className="bg-bg">
        <Container className="grid gap-10 py-14 md:py-20 lg:grid-cols-12 lg:gap-x-12">
          <div className="lg:col-span-3">
            <div className="lg:sticky lg:top-24">
              <h2 id="steps-title" className="font-mono text-xs tracking-wide text-fg-muted">
                Six steps, every project
              </h2>
              <ol className="mt-5 grid" data-steps="y">
                {steps.map((s, i) => (
                  <li key={s.id} className="relative grid grid-cols-[2rem_1fr] gap-x-3 pb-5 last:pb-0">
                    <span aria-hidden="true" className="relative flex justify-center">
                      <span
                        className={cn(
                          "relative z-10 grid size-8 place-items-center rounded-full border font-mono text-xs",
                          i === steps.length - 1
                            ? "border-charcoal-950 bg-lime text-charcoal-950"
                            : "border-border-strong bg-bg",
                        )}
                      >
                        {i + 1}
                      </span>
                      {i < steps.length - 1 && (
                        <span
                          data-connector
                          className="absolute top-8 bottom-[-1.25rem] left-1/2 w-px bg-border-strong"
                        />
                      )}
                    </span>
                    <a href={`#${s.id}`} className="min-h-8 pt-1 font-medium hover:text-accent-fg">
                      {s.name}
                    </a>
                  </li>
                ))}
              </ol>
            </div>
          </div>

          <div className="grid gap-0 lg:col-span-9">
            {steps.map((s, i) => (
              <section
                key={s.id}
                id={s.id}
                aria-labelledby={`${s.id}-title`}
                className="grid scroll-mt-24 gap-8 border-t border-border-strong pt-8 pb-14 md:pb-20 lg:grid-cols-9 lg:gap-x-10"
              >
                <div className="lg:col-span-4" data-reveal>
                  <p className="font-mono text-xs text-fg-muted">Step {i + 1}</p>
                  <h3
                    id={`${s.id}-title`}
                    className="mt-3 font-display text-[1.75rem] leading-[1.1] font-medium md:text-[2.25rem]"
                  >
                    {s.name}
                  </h3>
                  <p className="mt-3 text-lg">{s.line}</p>
                  {s.body.map((p) => (
                    <p key={p.slice(0, 24)} className="mt-4 text-fg-muted">
                      {p}
                    </p>
                  ))}
                  {s.id === "agreement" && (
                    <table className="mt-6 w-full text-left text-sm">
                      <caption className="mb-2 text-left font-mono text-xs text-fg-muted">
                        Bands shown with every agreement figure
                      </caption>
                      <tbody>
                        {bands.map((b) => (
                          <tr key={b.band} className="border-t border-border">
                            <th scope="row" className="py-2 pr-4 font-medium capitalize">
                              {b.band}
                            </th>
                            <td className="py-2 font-mono text-fg-muted">{b.range}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </div>
                <div className="lg:col-span-5">
                  {s.id === "deliver" ? (
                    <div className="grid gap-6">
                      <div>
                        <h4 className="font-mono text-xs tracking-wide text-fg-muted">{manifest.title}</h4>
                        <ul className="mt-3 grid gap-2 text-[0.9375rem]">
                          {manifest.fields.map((f) => (
                            <li key={f} className="border-t border-border pt-2">
                              {f}
                            </li>
                          ))}
                        </ul>
                      </div>
                      {visual.deliver}
                    </div>
                  ) : (
                    (visual[s.id] ?? <QualifyVisual />)
                  )}
                </div>
              </section>
            ))}
          </div>
        </Container>
      </section>

      {/* Trust score: a dark band so the formula reads as its own thing. */}
      <section id="trust" data-theme="dark" aria-labelledby="trust-title" className="scroll-mt-16 bg-bg">
        <Container className="grid gap-12 py-16 md:py-24 lg:grid-cols-12">
          <div className="lg:col-span-5" data-reveal>
            <Eyebrow>Contributors</Eyebrow>
            <h2
              id="trust-title"
              className="mt-4 font-display text-[2rem] leading-[1.08] font-medium tracking-[-0.02em] md:text-display-3"
            >
              {trust.title}
            </h2>
            <p className="mt-5 text-lg text-fg-muted">{trust.intro}</p>
          </div>
          <div className="lg:col-span-6 lg:col-start-7">
            <p className="font-mono text-sm text-fg-muted" aria-hidden="true">
              trust =
            </p>
            <ul className="mt-3 grid gap-3" aria-label="Trust score formula">
              {trust.parts.map((p) => (
                <li key={p.name} className="grid grid-cols-[4.5rem_1fr] items-center gap-4">
                  <span className="font-mono text-2xl tabular text-accent-fg">{p.weight.toFixed(1)}×</span>
                  <span className="grid gap-1.5">
                    <span>{p.name}</span>
                    <span aria-hidden="true" className="block h-1.5 rounded-full bg-sunken">
                      <span
                        className="block h-full rounded-full bg-accent-fg"
                        style={{ width: `${p.weight * 100}%` }}
                      />
                    </span>
                  </span>
                </li>
              ))}
            </ul>
            <ul className="mt-10 grid gap-4 text-fg-muted" data-reveal-group>
              {trust.rules.map((r) => (
                <li key={r} className="border-t border-border pt-4">
                  {r}
                </li>
              ))}
            </ul>
            <p className="mt-6 font-mono text-xs text-fg-muted">{trust.note}</p>
          </div>
        </Container>
      </section>

      <section id={security.id} aria-labelledby="security-title" className="scroll-mt-16 bg-bg">
        <Container className="py-16 md:py-24">
          <div className="grid gap-4 md:grid-cols-12" data-reveal>
            <Eyebrow className="md:col-span-3 md:pt-3">Security and privacy</Eyebrow>
            <h2
              id="security-title"
              className="font-display text-[2rem] leading-[1.08] font-medium tracking-[-0.02em] md:col-span-8 md:col-start-4 md:text-display-3"
            >
              {security.title}
            </h2>
          </div>
          <dl className="mt-12 grid border-t border-border-strong md:grid-cols-12">
            {security.items.map((item) => (
              <div
                key={item.name}
                className="grid gap-2 border-b border-border py-6 md:col-span-9 md:col-start-4 md:grid-cols-9 md:gap-6"
              >
                <dt className="font-display text-xl font-medium md:col-span-3">{item.name}</dt>
                <dd className="text-fg-muted md:col-span-6">{item.text}</dd>
              </div>
            ))}
          </dl>
          <p className="mt-6 text-sm text-fg-muted md:ml-[25%]">{security.note}</p>
        </Container>
      </section>

      <CtaBand title={qualityPage.cta.title} primary={qualityPage.cta.primary} />
      <PageMotion />
    </>
  );
}

/** Qualify: a test result against its pass mark (fictional). */
function QualifyVisual() {
  const score = 0.85;
  const pass = 0.8;
  return (
    <figure className="rounded-card border border-border bg-surface p-5 md:p-6">
      <figcaption className="mb-4 flex justify-between font-mono text-[0.6875rem] tracking-wider text-fg-muted uppercase">
        <span>Qualification test · Kiswahili classification</span>
        <span>Worked example</span>
      </figcaption>
      <div className="relative h-3 rounded-full bg-sunken" aria-hidden="true">
        <div className="h-full rounded-full bg-accent-fg" style={{ width: `${score * 100}%` }} />
        <div className="absolute -top-1.5 h-6 w-px bg-fg" style={{ left: `${pass * 100}%` }} />
      </div>
      <dl className="mt-4 grid grid-cols-3 gap-4 text-sm">
        <div>
          <dt className="text-fg-muted">Score</dt>
          <dd className="font-mono text-lg tabular">17 / 20</dd>
        </div>
        <div>
          <dt className="text-fg-muted">Pass mark</dt>
          <dd className="font-mono text-lg tabular">16 / 20</dd>
        </div>
        <div>
          <dt className="text-fg-muted">Next</dt>
          <dd className="font-medium">Ops review</dd>
        </div>
      </dl>
    </figure>
  );
}
