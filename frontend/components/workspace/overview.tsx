"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Card, EmptyState, Skeleton } from "@/components/ui/display";
import { api, type AttemptSummary, type ContributorSummary } from "@/lib/api/client";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { ClientHome } from "./client-home";
import { PageTitle, useCan } from "./workspace";

function Count({ label, value, href }: { label: string; value: number | null; href: string }) {
  return (
    <Link href={href} className="group">
      <Card interactive className="grid gap-2 p-5">
        <span className="text-sm text-fg-muted">{label}</span>
        <span className="flex items-end justify-between">
          {value == null ? (
            <Skeleton className="h-9 w-12" />
          ) : (
            <span className="font-mono text-3xl font-medium tabular">{value}</span>
          )}
          <ArrowRight
            aria-hidden="true"
            className="size-4 text-fg-muted transition-transform group-hover:translate-x-1"
          />
        </span>
      </Card>
    </Link>
  );
}

export function Overview() {
  const user = useUser();
  const can = useCan();
  const ops = can("contributor.approve");
  const queue = useLoad(() =>
    ops
      ? api<{ attempts: AttemptSummary[] }>("GET", "/v1/qualification/attempts").then(
          (r) => r.attempts.length,
        )
      : Promise.resolve(0),
  );
  const applicants = useLoad(() =>
    can("contributor.approve", "contributor.pause")
      ? Promise.all(
          (["applied", "qualifying"] as const).map((s) =>
            api<{ contributors: ContributorSummary[] }>("GET", `/v1/contributors?status=${s}`).then(
              (r) => r.contributors.length,
            ),
          ),
        ).then(([a, q]) => a + q)
      : Promise.resolve(0),
  );

  return (
    <>
      <PageTitle title={`Hello, ${user.display_name.split(/\s+/)[0]}`}>
        {user.kind === "staff" ? "What needs attention today." : "Your organisation's work on MezAI."}
      </PageTitle>
      {user.kind === "client" ? (
        <ClientHome />
      ) : ops ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Count
            label="Qualification attempts to review"
            value={queue.status === "ready" ? queue.data : null}
            href="/w/contributors"
          />
          <Count
            label="Applicants not yet active"
            value={applicants.status === "ready" ? applicants.data : null}
            href="/w/contributors?tab=all"
          />
        </div>
      ) : (
        <EmptyState title="Nothing here for your role yet">
          More of the workspace opens in the coming stages.
        </EmptyState>
      )}
    </>
  );
}
