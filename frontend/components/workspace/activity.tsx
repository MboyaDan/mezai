"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, EmptyState, Skeleton } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/input";
import { api, ApiError, type ActivityEntry, type ActivityList, type Organisation } from "@/lib/api/client";
import { formatDay } from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import { activityText } from "@/lib/portal";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { PageTitle } from "./workspace";

const time = (iso: string) =>
  new Date(iso).toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Africa/Nairobi",
  });
const nairobiDay = (iso: string) =>
  new Date(new Date(iso).getTime() + 3 * 3600_000).toISOString().slice(0, 10);

/** Entries grouped by Nairobi day, newest first. */
function byDay(entries: ActivityEntry[]): Array<[string, ActivityEntry[]]> {
  const out: Array<[string, ActivityEntry[]]> = [];
  for (const e of entries) {
    const d = nairobiDay(e.at);
    if (out.length && out[out.length - 1][0] === d) out[out.length - 1][1].push(e);
    else out.push([d, [e]]);
  }
  return out;
}

export function ActivityLog({ orgId, limit }: { orgId?: string; limit?: number }) {
  const query = orgId ? `?org_id=${orgId}` : "";
  const first = useLoad(() => api<ActivityList>("GET", `/v1/activity${query}`), query);
  const [older, setOlder] = useState<ActivityList[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  if (first.status === "loading") return <Skeleton className="h-60 w-full" />;
  if (first.status === "error") return <Alert tone="danger" title={errorMessage(first.error)} />;
  const pages = [first.data, ...older];
  const all = pages.flatMap((p) => p.entries);
  const entries = limit ? all.slice(0, limit) : all;
  const cursor = pages[pages.length - 1].before;

  if (entries.length === 0) {
    return (
      <EmptyState title="Nothing yet">
        Projects, datasets, deliverables, requests and invitations appear here as they happen.
      </EmptyState>
    );
  }

  const more = async () => {
    if (!cursor) return;
    setBusy(true);
    setError(null);
    try {
      const sep = query ? "&" : "?";
      const p = await api<ActivityList>(
        "GET",
        `/v1/activity${query}${sep}before=${encodeURIComponent(cursor)}`,
      );
      setOlder((o) => [...o, p]);
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="grid gap-6">
      {byDay(entries).map(([day, list]) => (
        <section key={day} aria-label={formatDay(day)} className="grid gap-2">
          <h2 className="text-sm font-semibold text-fg-muted">{formatDay(day)}</h2>
          <ol className="grid divide-y divide-border rounded-card border border-border bg-surface">
            {list.map((e) => (
              <li key={e.id} className="grid gap-x-4 gap-y-0.5 px-4 py-3 text-sm sm:grid-cols-[3.5rem_1fr]">
                <time dateTime={e.at} className="font-mono text-fg-muted tabular">
                  {time(e.at)}
                </time>
                <p>
                  <span className={e.actor.kind === "member" ? "font-medium" : "font-medium text-fg-muted"}>
                    {e.actor.name}
                  </span>{" "}
                  {activityText(e)}
                  {e.project_id && e.project_name && (
                    <>
                      {" · "}
                      <Link href={`/w/projects/${e.project_id}`} className="underline underline-offset-4">
                        {e.project_name}
                      </Link>
                    </>
                  )}
                </p>
              </li>
            ))}
          </ol>
        </section>
      ))}
      {error && <Alert tone="danger" title={errorMessage(error)} />}
      {!limit && cursor && (
        <div>
          <Button variant="secondary" onClick={more} disabled={busy}>
            {busy ? "Loading…" : "Show older activity"}
          </Button>
        </div>
      )}
    </div>
  );
}

/** /w/activity: the organisation's log (clients); Ops pick an organisation. */
export function ActivityPage() {
  const user = useUser();
  const staff = user.kind === "staff";
  const orgs = useLoad(() =>
    staff
      ? api<{ organisations: Organisation[] }>("GET", "/v1/organisations").then((r) => r.organisations)
      : Promise.resolve([] as Organisation[]),
  );
  const [org, setOrg] = useState("");
  const chosen = org || (orgs.status === "ready" ? (orgs.data[0]?.id ?? "") : "");

  return (
    <>
      <PageTitle title="Activity">
        {staff
          ? "What a client organisation sees in its activity log. Your actions appear to them as “MezAI team”."
          : "What has happened in your organisation's projects. Actions by the MezAI team are shown as “MezAI team”."}
      </PageTitle>
      {staff ? (
        orgs.status === "ready" ? (
          <div className="grid gap-6">
            <Field label="Organisation" className="max-w-sm">
              <Select value={chosen} onChange={(e) => setOrg(e.target.value)}>
                {orgs.data.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </Select>
            </Field>
            {chosen && <ActivityLog key={chosen} orgId={chosen} />}
          </div>
        ) : orgs.status === "error" ? (
          <Alert tone="danger" title={errorMessage(orgs.error)} />
        ) : (
          <Skeleton className="h-60 w-full" />
        )
      ) : (
        <ActivityLog />
      )}
    </>
  );
}
