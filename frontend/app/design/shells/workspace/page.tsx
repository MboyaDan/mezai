import { WorkspaceShell } from "@/components/shells/workspace-shell";
import { Button } from "@/components/ui/button";
import { Breadcrumb, Card, EmptyState, StatusPill } from "@/components/ui/display";

export default function WorkspaceShellPreview() {
  return (
    <WorkspaceShell
      user={{ name: "Sample Ops Manager", initials: "OM", role: "Ops Manager" }}
      orgLabel="ELS · Sample"
    >
      <Breadcrumb
        items={[
          { label: "Projects", href: "/design/shells/workspace" },
          { label: "Pairwise: farming advice (Sample)" },
        ]}
      />
      <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
        <div className="grid gap-2">
          <h1 className="font-display text-2xl font-semibold md:text-[1.75rem]">
            Pairwise: farming advice (Sample)
          </h1>
          <p className="flex flex-wrap items-center gap-3 text-sm text-fg-muted">
            <StatusPill status="success">Live</StatusPill> Deadline 30 Nov
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm">
            Pause
          </Button>
          <Button size="sm">Export</Button>
        </div>
      </div>
      <Card className="mt-8">
        <EmptyState className="border-0" title="No items need review">
          Items with no majority or a wide score spread appear here.
        </EmptyState>
      </Card>
    </WorkspaceShell>
  );
}
