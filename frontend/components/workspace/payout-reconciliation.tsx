"use client";

import Link from "next/link";
import { Alert, Breadcrumb, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { Table, type Column } from "@/components/ui/table";
import { api } from "@/lib/api/client";
import {
  b2cMode,
  batchLabel,
  exceptionLabel,
  payoutError,
  periodText,
  type BatchStatus,
  type ReconBatch,
  type ReconException,
  type Reconciliation,
} from "@/lib/payouts";
import { formatKes } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { PageTitle } from "./workspace";

const kes = (cents: number) => (
  <span className="font-mono whitespace-nowrap tabular">{formatKes(cents, "KES")}</span>
);

/** /w/payouts/reconciliation: per M-Pesa batch, lines, payments and ledger agree; and what Ops must look at. */
export function ReconciliationPage() {
  const r = useLoad(() => api<Reconciliation>("GET", "/v1/payouts/reconciliation"));
  if (r.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (r.status === "error") return <Alert tone="danger" title={payoutError(r.error)} />;
  const { b2c, batches, exceptions } = r.data;
  const mode = b2cMode(b2c);
  const unbalanced = batches.filter((b) => !b.balanced).length;

  return (
    <div className="grid gap-10">
      <div className="grid gap-3">
        <Breadcrumb items={[{ label: "Payouts", href: "/w/payouts" }, { label: "M-Pesa reconciliation" }]} />
        <PageTitle title="M-Pesa reconciliation">
          For every batch paid through M-Pesa, three totals must agree: lines recorded paid, payments M-Pesa
          confirmed, and payout entries in the earnings ledger. First, anything that needs a person.
        </PageTitle>
        <Alert tone={mode.real ? "info" : "warning"} title={`Automatic M-Pesa payments: ${mode.label}`}>
          {mode.note}
        </Alert>
      </div>

      <section aria-labelledby="exceptions-h" className="grid gap-3">
        <h2 id="exceptions-h" className="font-display text-lg font-semibold">
          Needs a person
          {exceptions.length > 0 && <span className="text-fg-muted"> ({exceptions.length})</span>}
        </h2>
        {exceptions.length === 0 ? (
          <EmptyState title="Nothing to look at">
            No payment is waiting too long, and every M-Pesa message matched a payment we sent.
          </EmptyState>
        ) : (
          <ExceptionTable rows={exceptions} />
        )}
      </section>

      <section aria-labelledby="recon-h" className="grid gap-3">
        <h2 id="recon-h" className="font-display text-lg font-semibold">
          Batches paid through M-Pesa
        </h2>
        {batches.length > 0 && (
          <p className="text-sm text-fg-muted">
            {unbalanced === 0
              ? batches.length === 1
                ? "The batch agrees."
                : `All ${batches.length} batches agree.`
              : `${unbalanced} of ${batches.length} ${batches.length === 1 ? "batch does" : "batches do"} not agree yet. A batch still being paid agrees once its results are in.`}
          </p>
        )}
        {batches.length === 0 ? (
          <EmptyState title="No batch has been paid through M-Pesa yet">
            Batches paid with the bulk payment file are reconciled on their own pages.
          </EmptyState>
        ) : (
          <BatchTable rows={batches} />
        )}
      </section>
    </div>
  );
}

function ExceptionTable({ rows }: { rows: ReconException[] }) {
  const columns: Column<ReconException>[] = [
    {
      key: "kind",
      header: "What",
      sortValue: (e) => e.kind,
      cell: (e) => (
        <span className="grid justify-items-start gap-1">
          <StatusPill status={e.kind === "stuck" ? "warning" : "danger"}>{exceptionLabel[e.kind]}</StatusPill>
          <span className="max-w-80 text-xs text-fg-muted">{e.note}</span>
        </span>
      ),
    },
    {
      key: "who",
      header: "Contributor",
      cell: (e) => <span className="font-mono text-sm">{e.public_code ?? "Unknown"}</span>,
    },
    {
      key: "amount",
      header: "Amount",
      numeric: true,
      sortValue: (e) => e.amount_cents ?? 0,
      cell: (e) =>
        e.amount_cents === null ? <span className="text-fg-muted">None</span> : kes(e.amount_cents),
    },
    {
      key: "at",
      header: "When",
      className: "hidden md:table-cell",
      sortValue: (e) => e.at,
      cell: (e) => <span className="text-sm whitespace-nowrap">{shortDate(e.at)}</span>,
    },
    {
      key: "batch",
      header: <span className="sr-only">Batch</span>,
      cell: (e) =>
        e.batch_id && (
          <Link href={`/w/payouts/${e.batch_id}`} className="text-sm underline underline-offset-4">
            Open batch<span className="sr-only"> for {e.public_code ?? "this payment"}</span>
          </Link>
        ),
    },
  ];
  return (
    <Table
      caption="Needs a person"
      hideCaption
      columns={columns}
      rows={rows}
      rowKey={(e) => `${e.kind}:${e.transaction_id ?? ""}:${e.callback_id ?? ""}`}
      empty="Nothing to look at."
    />
  );
}

function BatchTable({ rows }: { rows: ReconBatch[] }) {
  const columns: Column<ReconBatch>[] = [
    {
      key: "period",
      header: "Batch",
      sortValue: (b) => b.period_end,
      cell: (b) => (
        <span className="grid gap-0.5">
          <Link
            href={`/w/payouts/${b.id}`}
            className="font-medium underline underline-offset-4 md:whitespace-nowrap"
          >
            {periodText(b.period_start, b.period_end)}
          </Link>
          <span className="text-xs text-fg-muted">
            {batchLabel[b.status as BatchStatus] ?? b.status} · {b.paid_lines} paid, {b.failed_lines} failed
            {b.pending_lines > 0 && `, ${b.pending_lines} waiting`}
          </span>
        </span>
      ),
    },
    { key: "paid", header: "Lines paid", numeric: true, cell: (b) => kes(b.paid_cents) },
    {
      key: "mpesa",
      header: "M-Pesa confirmed",
      numeric: true,
      className: "hidden md:table-cell",
      cell: (b) => kes(b.succeeded_cents),
    },
    {
      key: "ledger",
      header: "Ledger",
      numeric: true,
      className: "hidden md:table-cell",
      cell: (b) => kes(b.ledger_cents),
    },
    {
      key: "balanced",
      header: "Agrees",
      sortValue: (b) => (b.balanced ? 1 : 0),
      cell: (b) => (
        <span className="grid justify-items-start gap-0.5">
          <StatusPill status={b.balanced ? "success" : "danger"}>
            {b.balanced ? "Agrees" : "Does not agree"}
          </StatusPill>
          {/* On a phone the two hidden totals are folded in here. */}
          {!b.balanced && (
            <span className="text-xs text-fg-muted md:hidden">
              M-Pesa {formatKes(b.succeeded_cents, "KES")}, ledger {formatKes(b.ledger_cents, "KES")}
            </span>
          )}
          {b.open_transactions > 0 && (
            <span className="text-xs text-fg-muted">
              {b.open_transactions} {b.open_transactions === 1 ? "payment" : "payments"} without a result
            </span>
          )}
        </span>
      ),
    },
  ];
  return (
    <Table
      caption="Batches paid through M-Pesa"
      hideCaption
      columns={columns}
      rows={rows}
      rowKey={(b) => b.id}
      empty="No batches."
    />
  );
}
