import type { Metadata } from "next";
import { Download } from "lucide-react";
import { Container, SectionTitle } from "@/components/public/layout";
import { PageHeader, titleText } from "@/components/public/page-header";
import { CtaBand } from "@/components/public/cta-band";
import { Stat, StatGroup } from "@/components/ui/data";
import { proofPage } from "@/content/proof";
import { fileSize, loadProof, percent, proofDir, proofFiles, type ProofStat } from "@/lib/proof";

export const metadata: Metadata = {
  title: "Sample delivery",
  description: proofPage.intro,
  alternates: { canonical: "/proof" },
  openGraph: { title: titleText(proofPage.title) },
};

const kappaName = (s: ProofStat) => (s.statistic === "cohen_kappa" ? "Cohen's kappa" : "Fleiss' kappa");
const isKappa = (s: ProofStat) => s.statistic === "fleiss_kappa" || s.statistic === "cohen_kappa";

/** A framed piece of the sample package, labelled as fictional (honesty rule 3). */
function DemoFrame({ name, children }: { name: string; children: React.ReactNode }) {
  return (
    <figure data-demo className="overflow-hidden rounded-card border border-border bg-surface">
      <figcaption
        data-theme="dark"
        className="flex items-center justify-between gap-4 bg-bg px-4 py-2 font-mono text-[0.6875rem] tracking-wider text-fg-muted uppercase"
      >
        <span className="truncate normal-case">{name}</span>
        <span className="shrink-0 text-accent-fg">{proofPage.label}</span>
      </figcaption>
      {children}
    </figure>
  );
}

export default function ProofPage() {
  const { manifest: m, manifestBytes, records } = loadProof();
  const { files, manifest: mc, records: rc, verify, cta } = proofPage;

  const overall = m.agreement.filter((s) => s.label === null);
  const kappa = overall.find(isKappa);
  const agreed = overall.find((s) => s.statistic === "percent");
  const labels = [...new Set(m.agreement.flatMap((s) => (s.label ? [s.label] : [])))];
  const byLabel = (label: string, stat: (s: ProofStat) => boolean) =>
    m.agreement.find((s) => s.label === label && stat(s));
  const dist = m.gold_accuracy.distribution;
  const reasons = Object.entries(m.review.reasons)
    .map(([r, n]) => `${n} ${mc.reasons[r] ?? r}`)
    .join(", ");

  const downloads = [
    {
      kind: "manifest" as const,
      name: proofFiles.manifest,
      bytes: manifestBytes,
      records: null,
      sha256: null,
    },
    ...m.files.map((f) => ({
      kind: f.format as "jsonl" | "csv",
      name: f.name,
      bytes: f.bytes,
      records: f.records,
      sha256: f.sha256,
    })),
  ];
  const csv = m.files.find((f) => f.format === "csv")!;

  return (
    <>
      <PageHeader
        eyebrow={proofPage.eyebrow}
        title={proofPage.title}
        intro={proofPage.intro}
        aside={
          <p className="border-l-2 border-border-strong pl-4 text-[0.9375rem] text-fg-muted">
            <span className="font-mono text-xs tracking-wider text-fg uppercase">{proofPage.label}</span>
            <br />
            {proofPage.disclaimer}
          </p>
        }
      />

      <section aria-labelledby="files-title" className="bg-bg">
        <Container className="py-16 md:py-24">
          <SectionTitle id="files-title" className="max-w-3xl lg:text-display-3">
            {files.title}
          </SectionTitle>
          <p className="mt-5 max-w-2xl text-lg text-fg-muted">{files.intro}</p>
          <ul className="mt-10 grid gap-4 md:grid-cols-3">
            {downloads.map((d) => (
              <li key={d.kind} className="flex flex-col rounded-card border border-border bg-surface p-5">
                <h3 className="font-display text-xl font-medium">{files.kinds[d.kind].name}</h3>
                <p className="mt-2 text-fg-muted">{files.kinds[d.kind].text}</p>
                <p className="mt-5 font-mono text-xs break-all text-fg">{d.name}</p>
                <p className="mt-1 font-mono text-xs text-fg-muted">
                  {fileSize(d.bytes)}
                  {d.records !== null && ` · ${d.records} records`}
                </p>
                {d.sha256 && (
                  <p className="mt-1 font-mono text-[0.6875rem] break-all text-fg-muted">sha256 {d.sha256}</p>
                )}
                <a
                  href={`${proofDir}/${d.name}`}
                  download
                  className="mt-auto inline-flex items-center gap-2 pt-5 font-medium underline decoration-border underline-offset-[0.3em] hover:decoration-accent-fg"
                >
                  <Download aria-hidden="true" className="size-4" />
                  Download {files.kinds[d.kind].name}
                </a>
              </li>
            ))}
          </ul>
        </Container>
      </section>

      <section aria-labelledby="manifest-title" className="border-t border-border bg-surface">
        <Container className="py-16 md:py-24">
          <SectionTitle id="manifest-title" className="max-w-3xl lg:text-display-3">
            {mc.title}
          </SectionTitle>
          <p className="mt-5 max-w-2xl text-lg text-fg-muted">{mc.intro}</p>
          <div className="mt-10">
            <DemoFrame name={`${m.project.name} · ${m.project.organisation} · version ${m.version}`}>
              <div className="bg-bg p-5 md:p-8">
                <StatGroup className="lg:grid-cols-4">
                  <Stat
                    label={mc.items}
                    value={`${m.items.delivered} of ${m.items.total}`}
                    caption={`${m.items.consensus} by consensus, ${m.items.adjudicated} decided by a reviewer, ${m.items.rejected} rejected at review`}
                  />
                  <Stat
                    label={mc.gold}
                    value={percent(m.gold_accuracy.accuracy)}
                    caption={`${m.gold_accuracy.correct} of ${m.gold_accuracy.results} gold answers right; per contributor ${percent(dist.min)} to ${percent(dist.max)}, median ${percent(dist.median)}`}
                  />
                  {kappa && (
                    <Stat
                      label={mc.agreement}
                      value={`κ\u00a0${kappa.value.toFixed(2)}`}
                      caption={`${kappa.band ? `${kappa.band[0].toUpperCase()}${kappa.band.slice(1)}. ` : ""}${kappaName(kappa)} over ${kappa.items} items, ${kappa.raters} answers each${agreed ? `; ${percent(agreed.value)} raw agreement` : ""}`}
                    />
                  )}
                  <Stat
                    label={mc.review}
                    value={percent(m.review.review_rate)}
                    caption={`${m.review.routed} of ${m.review.completed} items: ${reasons}`}
                  />
                </StatGroup>

                <div className="mt-10 overflow-x-auto" role="region" aria-label={mc.perLabel} tabIndex={0}>
                  <table className="w-full min-w-[28rem] text-left text-[0.9375rem]">
                    <caption className="pb-3 text-left font-mono text-xs tracking-wide text-fg-muted">
                      {mc.perLabel}
                    </caption>
                    <thead>
                      <tr className="border-b border-border-strong text-sm text-fg-muted">
                        <th scope="col" className="py-2 pr-4 font-normal">
                          Label
                        </th>
                        <th scope="col" className="py-2 pr-4 text-right font-normal">
                          Raw agreement
                        </th>
                        <th scope="col" className="py-2 pr-4 text-right font-normal">
                          Kappa
                        </th>
                        <th scope="col" className="py-2 font-normal">
                          Band
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {labels.map((l) => {
                        const p = byLabel(l, (s) => s.statistic === "percent");
                        const k = byLabel(l, isKappa);
                        return (
                          <tr key={l} className="border-b border-border">
                            <th scope="row" className="py-2 pr-4 font-mono text-sm font-normal">
                              {l}
                            </th>
                            <td className="py-2 pr-4 text-right font-mono tabular">
                              {p ? percent(p.value) : "–"}
                            </td>
                            <td className="py-2 pr-4 text-right font-mono tabular">
                              {k ? k.value.toFixed(2) : "–"}
                            </td>
                            <td className="py-2 text-fg-muted">{k?.band ?? "–"}</td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
                <p className="mt-6 font-mono text-xs text-fg-muted">
                  Contributors: {m.contributors.ids.join(", ")} · exported{" "}
                  {m.exported_at.replace("T", " ").replace("Z", " UTC")}
                </p>
              </div>
            </DemoFrame>
          </div>
        </Container>
      </section>

      <section aria-labelledby="records-title" className="border-t border-border bg-bg">
        <Container className="py-16 md:py-24">
          <SectionTitle id="records-title" className="max-w-3xl lg:text-display-3">
            {rc.title}
          </SectionTitle>
          <p className="mt-5 max-w-2xl text-lg text-fg-muted">{rc.intro}</p>
          <div className="mt-10">
            <DemoFrame name={proofFiles.jsonl}>
              <div className="overflow-x-auto" role="region" aria-label="Delivered items" tabIndex={0}>
                <table className="w-full min-w-[46rem] text-left text-[0.9375rem]">
                  <caption className="sr-only">Delivered items in the sample package</caption>
                  <thead>
                    <tr className="border-b border-border-strong text-sm text-fg-muted">
                      <th scope="col" className="px-4 py-3 font-normal">
                        Your ID
                      </th>
                      <th scope="col" className="px-4 py-3 font-normal">
                        Message
                      </th>
                      <th scope="col" className="px-4 py-3 font-normal">
                        Final label
                      </th>
                      <th scope="col" className="px-4 py-3 font-normal">
                        Decided by
                      </th>
                      <th scope="col" className="px-4 py-3 text-right font-normal">
                        Agreement
                      </th>
                      <th scope="col" className="px-4 py-3 font-normal">
                        Contributors
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {records.map((r) => (
                      <tr key={r.item_id} className="border-b border-border last:border-b-0">
                        <th scope="row" className="px-4 py-3 font-mono text-xs font-normal text-fg-muted">
                          {r.external_id}
                        </th>
                        <td className="px-4 py-3">{r.input.text}</td>
                        <td className="px-4 py-3 font-mono text-sm">{r.answer.labels.join(", ")}</td>
                        <td className="px-4 py-3">{rc.decidedBy[r.decided_by] ?? r.decided_by}</td>
                        <td className="px-4 py-3 text-right font-mono tabular">{percent(r.agreement)}</td>
                        <td className="px-4 py-3 font-mono text-xs text-fg-muted">
                          {r.contributors.join(" ")}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </DemoFrame>
          </div>
        </Container>
      </section>

      <section aria-labelledby="verify-title" className="border-t border-border bg-surface">
        <Container className="grid gap-10 py-16 md:grid-cols-12 md:py-24">
          <div className="min-w-0 md:col-span-6">
            <h2
              id="verify-title"
              className="font-display text-[2rem] leading-[1.08] font-medium md:text-display-3"
            >
              {verify.title}
            </h2>
            <p className="mt-5 text-lg text-fg-muted">{verify.text}</p>
            <pre
              data-theme="dark"
              className="mt-6 overflow-x-auto rounded-card bg-bg p-4 font-mono text-xs leading-relaxed text-fg"
              tabIndex={0}
              aria-label="Checksum command and expected output"
            >
              <code>
                {`$ ${verify.command} ${csv.name}\n`}
                <span className="text-fg-muted">{`${csv.sha256}  ${csv.name}`}</span>
              </code>
            </pre>
          </div>
          <div className="min-w-0 md:col-span-5 md:col-start-8">
            <h3 className="font-mono text-xs tracking-wide text-fg-muted">{verify.notesTitle}</h3>
            <ul className="mt-3 grid gap-2 text-[0.9375rem]">
              {m.notes.map((n) => (
                <li key={n} className="border-t border-border pt-2">
                  {n}
                </li>
              ))}
            </ul>
          </div>
        </Container>
      </section>

      <CtaBand title={cta.title} primary={cta.primary} secondary={cta.secondary} />
    </>
  );
}
