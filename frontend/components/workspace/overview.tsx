"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Card, EmptyState, Skeleton } from "@/components/ui/display";
import { api } from "@/lib/api/client";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { ClientHome } from "./client-home";
import { OpsHome } from "./ops-home";
import { PageTitle, useCan } from "./workspace";

/** A reviewer's home: the items waiting in their projects. */
function ReviewerHome() {
  const queue = useLoad(() =>
    api<{ items: unknown[] }>("GET", "/v1/review/items").then((r) => r.items.length),
  );
  return (
    <Link href="/w/review" className="group max-w-sm">
      <Card interactive className="grid gap-2 p-5">
        <span className="text-sm text-fg-muted">Items waiting for review in your projects</span>
        <span className="flex items-end justify-between">
          {queue.status === "ready" ? (
            <span className="font-mono text-3xl font-medium tabular">{queue.data}</span>
          ) : (
            <Skeleton className="h-9 w-12" />
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

  return (
    <>
      <PageTitle title={`Hello, ${user.display_name.split(/\s+/)[0]}`}>
        {user.kind === "staff" ? "What needs attention today." : "Your organisation's work on MezAI."}
      </PageTitle>
      {user.kind === "client" ? (
        <ClientHome />
      ) : can("project.manage") ? (
        <OpsHome />
      ) : can("review.read") ? (
        <ReviewerHome />
      ) : (
        <EmptyState title="Nothing here for your role yet">
          More of the workspace opens in the coming stages.
        </EmptyState>
      )}
    </>
  );
}
