import { ContributorShell } from "@/components/shells/contributor-shell";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, EmptyState, StatusPill } from "@/components/ui/display";

export default function ContributorShellPreview() {
  return (
    <ContributorShell initials="SC">
      <h1 className="font-display text-[1.75rem] font-medium tracking-[-0.02em] md:text-[2.5rem]">
        Habari, Sample Contributor 014
      </h1>
      <p className="mt-3 flex flex-wrap items-center gap-3 text-fg-muted">
        <StatusPill status="neutral">Qualifying</StatusPill> Swahili, English
      </p>
      <div className="mt-8 grid gap-6 lg:grid-cols-[7fr_5fr]">
        <EmptyState
          title="Take your first qualification test"
          action={<Button arrow>Start the Kiswahili test</Button>}
        >
          Projects appear here once you pass a test for one of your skills. It takes about 15 minutes.
        </EmptyState>
        <Card>
          <CardHeader title="Trust score" meta="Building" />
          <p className="px-5 pt-2 pb-5 text-sm text-fg-muted">
            Your score appears after 20 checked tasks. Until then you see more quality checks than usual.
          </p>
        </Card>
      </div>
    </ContributorShell>
  );
}
