"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Stat, StatGroup } from "@/components/ui/data";
import { Alert, Skeleton, StatusPill } from "@/components/ui/display";
import { Table, type Column } from "@/components/ui/table";
import { api, type ApiError, type EarningsSummary, type Payout, type StatementLine } from "@/lib/api/client";
import {
  formatDay,
  lineLabel,
  mainTotals,
  payoutLabel,
  payoutNote,
  payoutTone,
  signedKes,
} from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import { formatKes } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";

type Page = { lines: StatementLine[]; before: string | null };

const h2 = "font-display text-xl font-semibold";

/** /c/earnings: totals, the statement and payout history, all from the ledger (D-119). */
export function EarningsPage() {
  const summary = useLoad(() => api<EarningsSummary>("GET", "/v1/me/earnings"));
  const payouts = useLoad(() => api<{ payouts: Payout[] }>("GET", "/v1/me/payouts").then((r) => r.payouts));

  return (
    <div className="grid gap-12">
      <header className="grid gap-2">
        <h1 className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em] md:text-display-3">
          Earnings
        </h1>
        <p className="max-w-2xl text-fg-muted">
          Every accepted task adds to your balance at the project&apos;s rate. Every figure here is added up
          from your ledger, so it always matches what we pay.
        </p>
      </header>

      {summary.status === "loading" && <Skeleton className="h-32 w-full" />}
      {summary.status === "error" && <Alert tone="danger" title={errorMessage(summary.error)} />}
      {summary.status === "ready" && <Totals s={summary.data} />}

      <Statement />

      <section aria-labelledby="payouts-h" className="grid gap-4">
        <h2 id="payouts-h" className={h2}>
          Payouts
        </h2>
        {payouts.status === "loading" && <Skeleton className="h-24 w-full" />}
        {payouts.status === "error" && <Alert tone="danger" title={errorMessage(payouts.error)} />}
        {payouts.status === "ready" && <Payouts rows={payouts.data} />}
      </section>
    </div>
  );
}

function Totals({ s }: { s: EarningsSummary }) {
  const t = mainTotals(s);
  return (
    <section aria-labelledby="totals-h" className="grid gap-6">
      <h2 id="totals-h" className="sr-only">
        Totals
      </h2>
      <StatGroup className="border-y border-border py-6">
        <Stat
          label="Approved, not yet paid"
          value={formatKes(t.approved_cents)}
          size="lg"
          className="col-span-2 sm:col-span-1"
        />
        <Stat
          label="Pending"
          value={formatKes(t.pending_cents)}
          caption={`Estimate for ${t.pending_tasks} ${t.pending_tasks === 1 ? "task" : "tasks"} not yet decided`}
        />
        <Stat label="Paid to M-Pesa" value={formatKes(t.paid_cents)} />
        <Stat label="Earned in total" value={formatKes(t.earned_cents)} caption="Approved plus paid" />
      </StatGroup>
      <div className="grid gap-x-10 gap-y-3 text-sm text-fg-muted md:grid-cols-2">
        <p>{payoutNote(s)}</p>
        <p>
          Of your {s.tasks.submitted} submitted {s.tasks.submitted === 1 ? "task" : "tasks"}:{" "}
          {s.tasks.accepted} accepted, {s.tasks.waiting} waiting for a decision, {s.tasks.not_accepted} not
          accepted. A task that is not accepted earns nothing; the reviewer&apos;s reason is in your feedback.
        </p>
      </div>
    </section>
  );
}

// Three columns so the amount stays on screen at 360 px; the receipt or
// reason sits under the description.
const statementColumns: Column<StatementLine>[] = [
  {
    key: "day",
    header: "Day",
    cell: (l) => <span className="whitespace-nowrap">{formatDay(l.day)}</span>,
  },
  {
    key: "what",
    header: "What",
    cell: (l) => (
      <span className="grid gap-0.5">
        <span>{lineLabel(l)}</span>
        {l.project_name && <span className="text-xs text-fg-muted">{l.project_name}</span>}
        {l.mpesa_receipt && (
          <span className="text-xs text-fg-muted">
            Receipt <span className="font-mono">{l.mpesa_receipt}</span>
          </span>
        )}
        {l.reason && <span className="text-xs text-fg-muted">{l.reason}</span>}
      </span>
    ),
  },
  {
    key: "amount",
    header: "Amount",
    numeric: true,
    cell: (l) => <span className="whitespace-nowrap">{signedKes(l.amount_cents, l.currency)}</span>,
  },
];

/** The statement, newest first; "Show older" fetches the next 30 days with entries. */
function Statement() {
  const first = useLoad(() => api<Page>("GET", "/v1/me/earnings/statement"));
  const [older, setOlder] = useState<Page[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const pages = first.status === "ready" ? [first.data, ...older] : [];
  const lines = pages.flatMap((p) => p.lines);
  const cursor = pages.length ? pages[pages.length - 1].before : null;

  const more = async () => {
    if (!cursor) return;
    setBusy(true);
    setError(null);
    try {
      const p = await api<Page>("GET", `/v1/me/earnings/statement?before=${cursor}`);
      setOlder((o) => [...o, p]);
    } catch (e) {
      setError(e as ApiError);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-labelledby="statement-h" className="grid gap-4">
      <div className="grid gap-1">
        <h2 id="statement-h" className={h2}>
          Statement
        </h2>
        <p className="text-sm text-fg-muted">
          Accepted work is added up per day and project. Days are Nairobi time.
        </p>
      </div>
      {first.status === "loading" && <Skeleton className="h-40 w-full" />}
      {first.status === "error" && <Alert tone="danger" title={errorMessage(first.error)} />}
      {first.status === "ready" && (
        <>
          <Table
            caption="Statement"
            hideCaption
            columns={statementColumns}
            rows={lines}
            rowKey={(l) =>
              `${l.day}:${l.type}:${l.project_id ?? ""}:${l.amount_cents}:${l.mpesa_receipt ?? l.reason ?? ""}`
            }
            empty="Nothing yet. Your first accepted task will appear here."
          />
          {error && <Alert tone="danger" title={errorMessage(error)} />}
          {cursor && (
            <div>
              <Button variant="secondary" onClick={more} disabled={busy}>
                {busy ? "Loading…" : "Show older entries"}
              </Button>
            </div>
          )}
        </>
      )}
    </section>
  );
}

const payoutColumns: Column<Payout>[] = [
  {
    key: "period",
    header: "Work period",
    cell: (p) => (
      <span className="whitespace-nowrap">
        {formatDay(p.period_start)} – {formatDay(p.period_end)}
      </span>
    ),
  },
  { key: "amount", header: "Amount", numeric: true, cell: (p) => formatKes(p.amount_cents, p.currency) },
  {
    key: "status",
    header: "Status",
    cell: (p) => <StatusPill status={payoutTone[p.status]}>{payoutLabel[p.status]}</StatusPill>,
  },
  {
    key: "detail",
    header: "Receipt or reason",
    cell: (p) =>
      p.mpesa_receipt ? (
        <span className="font-mono text-xs">{p.mpesa_receipt}</span>
      ) : (
        <span className="text-fg-muted">{p.failure_reason ?? "–"}</span>
      ),
  },
  {
    key: "phone",
    header: "Sent to",
    cell: (p) => <span className="font-mono text-xs">{p.phone_masked}</span>,
  },
];

function Payouts({ rows }: { rows: Payout[] }) {
  return (
    <>
      <Table
        caption="Payouts"
        hideCaption
        columns={payoutColumns}
        rows={rows}
        rowKey={(p) => p.id}
        empty="No payouts yet. Payouts are made in batches once your approved balance reaches the minimum."
      />
      {rows.some((p) => p.status === "failed") && (
        <p className="text-sm text-fg-muted">
          A payment that did not go through stays in your approved balance, so it can be paid in a later
          batch. Check that your M-Pesa number on your profile is correct.
        </p>
      )}
    </>
  );
}
