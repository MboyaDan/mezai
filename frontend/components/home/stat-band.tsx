import { Container } from "@/components/public/layout";
import { getStats } from "@/lib/stats";

const dateFmt = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "Africa/Nairobi",
});
const numFmt = new Intl.NumberFormat("en-GB");

/**
 * Published statistics, each with its source and verification date. Renders
 * nothing until a real record exists (honesty rule 2).
 */
export async function StatBand() {
  const stats = await getStats();
  if (stats.length === 0) return null;
  return (
    <div className="border-t border-border">
      <Container className="py-10">
        <dl className="grid gap-x-10 gap-y-8 sm:grid-cols-2 lg:grid-cols-4">
          {stats.map((s) => (
            <div key={s.key} className="grid content-start gap-1" data-stat={s.key}>
              <dt className="text-sm text-fg-muted">{s.label}</dt>
              <dd className="font-mono text-4xl text-fg tabular">
                {numFmt.format(s.value)}
                {s.unit && <span className="ml-1.5 text-base text-fg-muted">{s.unit}</span>}
              </dd>
              <dd className="text-xs text-fg-muted">
                Source:{" "}
                {s.source_url ? (
                  <a href={s.source_url} className="underline underline-offset-2 hover:text-fg">
                    {s.source}
                  </a>
                ) : (
                  s.source
                )}
                , verified {dateFmt.format(new Date(s.verified_at))}
              </dd>
            </div>
          ))}
        </dl>
      </Container>
    </div>
  );
}
