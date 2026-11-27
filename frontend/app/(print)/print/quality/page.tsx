import type { Metadata } from "next";
import { Logo } from "@/components/brand/logo";
import { qualityPage } from "@/content/quality";
import { brand } from "@/lib/brand";
import { siteUrl } from "@/lib/site";

// "How MezAI assures quality", one A4 page for buyers (Stage 20a, D-172).
// Every sentence comes from content/quality.ts or lib/brand.ts, so the PDF
// says what the site says. `npm run pdf:quality` prints it; a full-stack
// test checks it still fits on one page.

export const metadata: Metadata = { title: qualityPage.onePager.title, robots: { index: false } };

const host = siteUrl.replace(/^https?:\/\//, "");

export default function Page() {
  const q = qualityPage;
  return (
    <main className="mx-auto grid w-[210mm] gap-[4mm] bg-surface px-[14mm] py-[10mm] text-[9pt] leading-snug text-fg">
      <style>{"@page { size: A4; margin: 0 } html, body { background: var(--color-surface, #fff) }"}</style>
      <header className="flex items-end justify-between border-b border-border pb-[3mm]">
        <div className="flex items-center gap-3">
          <Logo className="h-7 w-auto" />
          <span className="text-fg-muted">{brand.descriptor}</span>
        </div>
        <span className="font-mono text-[8pt] text-fg-muted">{q.eyebrow}</span>
      </header>

      <section className="grid gap-[2mm]">
        <h1 className="font-display text-[19pt] leading-tight font-medium tracking-[-0.02em]">
          {q.title.lead} {q.title.accent} {q.title.tail}
        </h1>
        <p className="max-w-[160mm] text-[10pt] text-fg-muted">{q.intro}</p>
      </section>

      <section aria-label="Steps" className="grid grid-cols-2 gap-x-[6mm] gap-y-[3mm]">
        {q.steps.map((s, i) => (
          <div key={s.id} className="grid content-start gap-[0.8mm] border-t border-border pt-[2mm]">
            <h2 className="font-display text-[11pt] font-semibold">
              <span className="mr-2 font-mono text-[8pt] text-fg-muted">0{i + 1}</span>
              {s.name}
            </h2>
            <p className="font-medium">{s.line}</p>
            <p className="text-fg-muted">{s.body[0]}</p>
          </div>
        ))}
      </section>

      <section className="grid grid-cols-[1fr_1.3fr] gap-[6mm] border-t border-border pt-[3mm]">
        <div className="grid content-start gap-[1.5mm]">
          <h2 className="font-display text-[11pt] font-semibold">{q.trust.title}</h2>
          <p className="text-fg-muted">{q.trust.intro}</p>
          <ul className="grid gap-[0.5mm]">
            {q.trust.parts.map((p) => (
              <li key={p.name}>
                <span className="font-mono">{p.weight.toFixed(1)}</span> × {p.name}
              </li>
            ))}
          </ul>
          <p className="text-fg-muted">{q.trust.rules[1]}</p>
          <p className="text-[8pt] text-fg-muted">{q.trust.note}</p>
        </div>
        <div className="grid content-start gap-[1.5mm]">
          <h2 className="font-display text-[11pt] font-semibold">{q.security.title}</h2>
          <ul className="grid gap-[1.2mm]">
            {q.security.items.map((s) => (
              <li key={s.name}>
                <strong className="font-semibold">{s.name}.</strong>{" "}
                <span className="text-fg-muted">{s.text}</span>
              </li>
            ))}
          </ul>
          <p className="font-medium">{q.security.note}</p>
        </div>
      </section>

      <footer className="flex items-end justify-between border-t border-border pt-[3mm]">
        <div className="grid gap-[0.5mm]">
          <p className="font-display text-[11pt] font-semibold">{q.cta.title}</p>
          <p>
            {q.cta.primary.label}: <span className="font-mono">{host}/contact</span>
          </p>
        </div>
        <p className="max-w-[70mm] text-right text-[8pt] text-fg-muted">
          {q.onePager.note} {brand.name} is operated by {brand.legalEntity}, {brand.city}.
        </p>
      </footer>
    </main>
  );
}
