"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { StatusBadge } from "@/components/contributor/status";
import { Alert, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { Tabs } from "@/components/ui/tabs";
import { labelFor } from "@/content/profile";
import { api, type AttemptSummary, type ContributorStatus, type ContributorSummary } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { identityLabel, identityPill } from "./identity-panel";
import { PageTitle, useCan } from "./workspace";

export const shortDate = (iso: string | null | undefined) =>
  iso ? new Date(iso).toLocaleDateString("en-KE", { day: "numeric", month: "short", year: "numeric" }) : "–";
export const pct = (n: number | null | undefined) => (n == null ? "–" : `${Math.round(n * 100)}%`);

function Queue() {
  const q = useLoad(() =>
    api<{ attempts: AttemptSummary[] }>("GET", "/v1/qualification/attempts").then((r) => r.attempts),
  );
  if (q.status === "loading") return <Skeleton className="h-40 w-full" />;
  if (q.status === "error") return <Alert tone="danger" title={errorMessage(q.error)} />;
  return (
    <Table
      caption="Qualification attempts waiting for review, oldest first"
      hideCaption
      rows={q.data}
      rowKey={(a) => a.id}
      empty="No attempts are waiting. New ones appear here as soon as a contributor submits."
      columns={[
        {
          key: "who",
          header: "Contributor",
          cell: (a) => (
            <span className="grid">
              <span className="font-mono text-xs text-fg-muted">{a.public_code}</span>
              <span>{a.display_name}</span>
            </span>
          ),
          sortValue: (a) => a.public_code,
        },
        { key: "test", header: "Test", cell: (a) => a.test_title, sortValue: (a) => a.test_title },
        {
          key: "score",
          header: "Score / pass mark",
          numeric: true,
          cell: (a) => `${pct(a.score)} / ${pct(a.pass_threshold)}`,
          sortValue: (a) => a.score ?? 0,
        },
        {
          key: "result",
          header: "Result",
          cell: (a) => (
            <StatusPill status={a.passed ? "success" : "warning"}>
              {a.passed ? "Passed" : "Below pass mark"}
            </StatusPill>
          ),
        },
        {
          key: "at",
          header: "Submitted",
          cell: (a) => shortDate(a.submitted_at),
          sortValue: (a) => a.submitted_at ?? "",
        },
        {
          key: "go",
          header: <span className="sr-only">Action</span>,
          cell: (a) => (
            <Link
              href={`/w/contributors/attempts/${a.id}`}
              className="font-medium underline underline-offset-4"
            >
              Review<span className="sr-only"> {a.public_code}</span>
            </Link>
          ),
        },
      ]}
    />
  );
}

/** Identities waiting for Ops: numbers given, not yet verified (D-078). */
function IdentityQueue() {
  const list = useLoad(() =>
    api<{ contributors: ContributorSummary[] }>("GET", "/v1/contributors?identity=pending").then((r) =>
      r.contributors.filter((c) => c.national_id_last3 && c.status !== "removed").reverse(),
    ),
  );
  if (list.status === "loading") return <Skeleton className="h-40 w-full" />;
  if (list.status === "error") return <Alert tone="danger" title={errorMessage(list.error)} />;
  return (
    <Table
      caption="Identities waiting for a check, oldest first"
      hideCaption
      rows={list.data}
      rowKey={(c) => c.id}
      empty="No identities are waiting. Contributors appear here once they give their National ID and M-Pesa numbers."
      columns={[
        {
          key: "who",
          header: "Contributor",
          cell: (c) => (
            <span className="grid">
              <span className="font-mono text-xs text-fg-muted">{c.public_code}</span>
              <span>{c.display_name}</span>
            </span>
          ),
          sortValue: (c) => c.public_code,
        },
        { key: "status", header: "Status", cell: (c) => <StatusBadge status={c.status} /> },
        {
          key: "id",
          header: "ID ending",
          cell: (c) => <span className="font-mono">{c.national_id_last3}</span>,
        },
        {
          key: "dupes",
          header: "Possible duplicates",
          numeric: true,
          cell: (c) =>
            c.possible_duplicates > 0 ? (
              <StatusPill status="warning">{c.possible_duplicates}</StatusPill>
            ) : (
              "None"
            ),
          sortValue: (c) => c.possible_duplicates,
        },
        {
          key: "go",
          header: <span className="sr-only">Action</span>,
          cell: (c) => (
            <Link href={`/w/contributors/${c.id}`} className="font-medium underline underline-offset-4">
              Check<span className="sr-only"> {c.public_code}</span>
            </Link>
          ),
        },
      ]}
    />
  );
}

const statuses: Array<ContributorStatus | ""> = ["", "applied", "qualifying", "active", "paused", "removed"];

function All() {
  const [status, setStatus] = useState<ContributorStatus | "">("");
  const list = useLoad(
    () =>
      api<{ contributors: ContributorSummary[] }>(
        "GET",
        `/v1/contributors${status ? `?status=${status}` : ""}`,
      ).then((r) => r.contributors),
    status,
  );
  return (
    <div className="grid gap-4">
      <Field label="Status" className="max-w-56">
        <Select value={status} onChange={(e) => setStatus(e.target.value as ContributorStatus | "")}>
          {statuses.map((s) => (
            <option key={s} value={s}>
              {s ? s[0].toUpperCase() + s.slice(1) : "All statuses"}
            </option>
          ))}
        </Select>
      </Field>
      {list.status === "loading" && <Skeleton className="h-40 w-full" />}
      {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
      {list.status === "ready" && (
        <Table
          caption="Contributors, newest first"
          hideCaption
          rows={list.data}
          rowKey={(c) => c.id}
          empty="No contributors with this status."
          columns={[
            {
              key: "code",
              header: "ID",
              cell: (c) => (
                <Link href={`/w/contributors/${c.id}`} className="font-mono underline underline-offset-4">
                  {c.public_code}
                </Link>
              ),
              sortValue: (c) => c.public_code,
            },
            { key: "name", header: "Name", cell: (c) => c.display_name, sortValue: (c) => c.display_name },
            {
              key: "status",
              header: "Status",
              cell: (c) => <StatusBadge status={c.status} />,
              sortValue: (c) => c.status,
            },
            {
              key: "identity",
              header: "Identity",
              cell: (c) => (
                <StatusPill status={identityPill(c.identity_status)}>
                  {identityLabel[c.identity_status]}
                </StatusPill>
              ),
              sortValue: (c) => c.identity_status,
            },
            { key: "langs", header: "Languages", cell: (c) => c.languages.map(labelFor).join(", ") || "–" },
            {
              key: "phone",
              header: "M-Pesa",
              cell: (c) => <span className="font-mono text-xs">{c.mpesa_phone_masked || "–"}</span>,
            },
            {
              key: "joined",
              header: "Joined",
              cell: (c) => shortDate(c.created_at),
              sortValue: (c) => c.created_at,
            },
          ]}
        />
      )}
    </div>
  );
}

export function Contributors() {
  const can = useCan();
  const params = useSearchParams();
  const approve = can("contributor.approve");
  const tabs = [
    ...(approve
      ? [
          { id: "queue", label: "Waiting for review", content: <Queue /> },
          { id: "identity", label: "Identity checks", content: <IdentityQueue /> },
        ]
      : []),
    { id: "all", label: "All contributors", content: <All /> },
  ];
  return (
    <>
      <PageTitle title="Contributors">Applicants, qualification approvals and status changes.</PageTitle>
      <Tabs
        label="Contributors"
        tabs={tabs}
        defaultTab={
          !approve
            ? "all"
            : params.get("tab") === "all" || params.get("tab") === "identity"
              ? params.get("tab")!
              : "queue"
        }
      />
    </>
  );
}
