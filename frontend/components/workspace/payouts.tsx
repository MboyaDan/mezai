"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type FormEvent } from "react";
import { Button, ButtonLink } from "@/components/ui/button";
import { Alert, Card, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Table, type Column } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { api, ApiError } from "@/lib/api/client";
import { formatDay } from "@/lib/earnings";
import {
  batchKey,
  batchLabel,
  batchTone,
  holdLabel,
  latestPeriodEnd,
  lineLabel,
  lineTone,
  nairobiToday,
  openStatuses,
  payoutError,
  periodText,
  resultsText,
  type PayoutBatch,
  type PayoutBatchSummary,
  type PayoutHeld,
  type PayoutLine,
  type PayoutPreview,
} from "@/lib/payouts";
import { formatKes } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { PageTitle, useCan } from "./workspace";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

type BatchList = { batches: PayoutBatchSummary[]; next_date: string | null };

/** /w/payouts: Ops prepare batches and set the next payout date; approvers find what waits for them. */
export function PayoutsPage() {
  const can = useCan();
  const create = can("payout.create");
  const list = useLoad(() => api<BatchList>("GET", "/v1/payouts/batches"));

  if (list.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (list.status === "error") return <Alert tone="danger" title={payoutError(list.error)} />;
  const { batches, next_date } = list.data;
  const open = batches.find((b) => openStatuses.includes(b.status));

  return (
    <div className="grid gap-10">
      <PageTitle title="Payouts">
        Contributors are paid by M-Pesa in batches. One person prepares a batch, a second person approves it,
        then the team pays it in the M-Pesa business portal and records each result here. Money leaves a
        contributor&apos;s balance only when their line is recorded paid.
      </PageTitle>
      {open ? (
        <OpenBatch batch={open} />
      ) : create ? (
        <NewBatch />
      ) : (
        <Alert tone="info" title="No batch is waiting for approval">
          Batches appear here when the operations team prepares one.
        </Alert>
      )}
      <section aria-labelledby="batches-h" className="grid gap-3">
        <h2 id="batches-h" className="font-display text-lg font-semibold">
          Batches
        </h2>
        {batches.length === 0 ? (
          <EmptyState title="No payout batches yet">
            {create ? "Preview the first batch above." : "Nothing has been paid through MezAI yet."}
          </EmptyState>
        ) : (
          <BatchTable batches={batches} />
        )}
      </section>
      {create && <NextDate initial={next_date} onSaved={(d) => list.set({ batches, next_date: d })} />}
    </div>
  );
}

function OpenBatch({ batch: b }: { batch: PayoutBatchSummary }) {
  return (
    <section aria-labelledby="open-h" className="grid gap-3">
      <h2 id="open-h" className="font-display text-lg font-semibold">
        Open batch
      </h2>
      <Card className="flex flex-wrap items-center justify-between gap-4 p-5">
        <div className="grid gap-1">
          <p className="font-semibold">{periodText(b.period_start, b.period_end)}</p>
          <p className="text-sm text-fg-muted">
            <span className="font-mono tabular">{formatKes(b.total_cents, b.currency)}</span> to{" "}
            {b.line_count} {b.line_count === 1 ? "contributor" : "contributors"}
            {b.status === "exported" && <> · {resultsText(b)}</>}
          </p>
          <p className="text-sm text-fg-muted">
            Only one batch is open at a time, so nobody is paid twice for the same earnings.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <StatusPill status={batchTone[b.status]}>{batchLabel[b.status]}</StatusPill>
          <ButtonLink href={`/w/payouts/${b.id}`} arrow>
            Open batch
          </ButtonLink>
        </div>
      </Card>
    </section>
  );
}

function NewBatch() {
  const router = useRouter();
  const toast = useToast();
  const latest = latestPeriodEnd();
  const [end, setEnd] = useState(latest);
  const [preview, setPreview] = useState<PayoutPreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const key = useRef<string | null>(null);

  const load = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      setPreview(
        await api<PayoutPreview>("GET", `/v1/payouts/preview?period_end=${encodeURIComponent(end)}`),
      );
      key.current = null;
    } catch (err) {
      setPreview(null);
      setError(payoutError(asError(err)));
    } finally {
      setBusy(false);
    }
  };

  const createBatch = async () => {
    if (!preview) return;
    key.current ??= batchKey(preview.period_end);
    setBusy(true);
    setError(null);
    try {
      const b = await api<PayoutBatch>(
        "POST",
        "/v1/payouts/batches",
        { period_end: preview.period_end },
        { "Idempotency-Key": key.current },
      );
      toast({ tone: "success", title: "Draft batch created" });
      router.push(`/w/payouts/${b.id}`);
    } catch (err) {
      setError(payoutError(asError(err)));
      setBusy(false);
    }
  };

  return (
    <section aria-labelledby="new-h" className="grid gap-3">
      <h2 id="new-h" className="font-display text-lg font-semibold">
        New batch
      </h2>
      <Card className="grid gap-5 p-5">
        <form onSubmit={load} className="flex flex-wrap items-end gap-3">
          <Field
            label="Period ends on"
            required
            hint="Earnings approved up to the end of this day. Yesterday at the latest."
            className="w-full max-w-xs"
          >
            <Input
              type="date"
              required
              max={latest}
              value={end}
              onChange={(e) => {
                setEnd(e.target.value);
                setPreview(null);
              }}
            />
          </Field>
          <Button type="submit" variant="secondary" disabled={busy || !end}>
            {busy && !preview ? "Checking…" : "Preview"}
          </Button>
        </form>
        {error && <Alert tone="danger" title={error} />}
        {preview && <PreviewPanel p={preview} busy={busy} onCreate={createBatch} />}
      </Card>
    </section>
  );
}

function PreviewPanel({ p, busy, onCreate }: { p: PayoutPreview; busy: boolean; onCreate: () => void }) {
  const heldCents = p.held.reduce((s, h) => s + h.owed_cents, 0);
  return (
    <div className="grid gap-5 border-t border-border pt-5">
      <dl className="grid gap-4 sm:grid-cols-3">
        <Figure label="Period" value={periodText(p.period_start, p.period_end)} />
        <Figure
          label={`To pay, ${p.lines.length} ${p.lines.length === 1 ? "contributor" : "contributors"}`}
          value={formatKes(p.total_cents)}
          mono
        />
        <Figure
          label={`Held, ${p.held.length} ${p.held.length === 1 ? "contributor" : "contributors"}`}
          value={formatKes(heldCents)}
          mono
        />
      </dl>
      <p className="text-sm text-fg-muted">
        Each person is paid the smaller of their balance now and at the end of the period, in whole shillings.
        Cents stay owed. The minimum payout is {formatKes(p.min_cents)}.
      </p>
      {p.lines.length > 0 && (
        <LinesTable caption="Contributors in this batch" lines={p.lines} showBalance={false} />
      )}
      {p.held.length > 0 && <HeldTable held={p.held} />}
      {p.lines.length === 0 ? (
        <Alert tone="info" title="Nobody is due a payout for this period" />
      ) : (
        <div>
          <Button onClick={onCreate} disabled={busy}>
            {busy ? "Creating…" : "Create draft batch"}
          </Button>
        </div>
      )}
    </div>
  );
}

export function Figure({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="grid gap-1">
      <dt className="text-sm text-fg-muted">{label}</dt>
      <dd className={mono ? "font-mono text-lg font-semibold tabular" : "font-semibold"}>{value}</dd>
    </div>
  );
}

function HeldTable({ held }: { held: PayoutHeld[] }) {
  const columns: Column<PayoutHeld>[] = [
    {
      key: "who",
      header: "Contributor",
      sortValue: (h) => h.public_code,
      cell: (h) => (
        <Who code={h.public_code} name={h.name} id={h.contributor_id}>
          <span className="text-xs sm:hidden">{holdLabel[h.reason]}</span>
        </Who>
      ),
    },
    {
      key: "reason",
      header: "Why held",
      className: "hidden sm:table-cell",
      sortValue: (h) => h.reason,
      cell: (h) => holdLabel[h.reason],
    },
    {
      key: "owed",
      header: "Owed",
      numeric: true,
      sortValue: (h) => h.owed_cents,
      cell: (h) => <span className="font-mono whitespace-nowrap tabular">{formatKes(h.owed_cents)}</span>,
    },
  ];
  return (
    <div className="grid gap-2">
      <h3 className="font-semibold">Held this time</h3>
      <p className="text-sm text-fg-muted">
        Their money stays owed and joins a later batch once the reason is cleared on their record.
      </p>
      <Table
        caption="Held contributors"
        hideCaption
        columns={columns}
        rows={held}
        rowKey={(h) => h.contributor_id}
        empty="Nobody is held."
      />
    </div>
  );
}

export function Who({
  code,
  name,
  id,
  phone,
  children,
}: {
  code: string;
  name: string;
  id: string;
  phone?: string;
  children?: React.ReactNode;
}) {
  return (
    <span className="grid gap-0.5">
      <Link href={`/w/contributors/${id}`} className="font-mono text-sm underline underline-offset-4">
        {code}
      </Link>
      <span className="text-xs text-fg-muted">{name}</span>
      {phone && (
        <span className="font-mono text-xs whitespace-nowrap text-fg-muted">
          <span className="sr-only">M-Pesa </span>
          {phone}
        </span>
      )}
      {children}
    </span>
  );
}

/** Lines for a preview (no balance column) or a batch. `action` adds a last column. */
export function LinesTable({
  caption,
  lines,
  showBalance,
  showResult,
  action,
}: {
  caption: string;
  lines: PayoutLine[];
  showBalance: boolean;
  showResult?: boolean;
  action?: (l: PayoutLine) => React.ReactNode;
}) {
  const columns: Column<PayoutLine>[] = [
    {
      key: "who",
      header: "Contributor",
      sortValue: (l) => l.public_code,
      cell: (l) => <Who code={l.public_code} name={l.name} id={l.contributor_id} phone={l.phone_masked} />,
    },
    {
      key: "amount",
      header: "Amount",
      numeric: true,
      sortValue: (l) => l.amount_cents,
      cell: (l) => (
        <span className="font-mono whitespace-nowrap tabular">{formatKes(l.amount_cents, l.currency)}</span>
      ),
    },
  ];
  if (showBalance) {
    columns.push({
      key: "balance",
      header: "Balance now",
      numeric: true,
      className: "hidden md:table-cell",
      cell: (l) => (
        <span className="grid justify-items-end gap-0.5">
          <span className="font-mono whitespace-nowrap tabular">
            {formatKes(l.balance_cents, l.currency)}
          </span>
          {l.status === "pending" && l.amount_cents > l.balance_cents && (
            <span className="text-xs font-semibold text-danger-fg">Less than the line</span>
          )}
        </span>
      ),
    });
  }
  if (showResult) {
    columns.push({
      key: "result",
      header: "Result",
      sortValue: (l) => l.status,
      cell: (l) => <LineResult l={l} />,
    });
  }
  if (action) columns.push({ key: "action", header: <span className="sr-only">Action</span>, cell: action });
  return (
    <Table
      caption={caption}
      hideCaption
      columns={columns}
      rows={lines}
      rowKey={(l) => l.contributor_id}
      empty="No lines."
    />
  );
}

function LineResult({ l }: { l: PayoutLine }) {
  return (
    <span className="grid justify-items-start gap-0.5">
      <StatusPill status={lineTone[l.status]}>{lineLabel[l.status]}</StatusPill>
      {l.mpesa_receipt && <span className="font-mono text-xs">{l.mpesa_receipt}</span>}
      {l.failure_reason && <span className="max-w-56 text-xs text-fg-muted">{l.failure_reason}</span>}
      {l.recorded_by_name && <span className="text-xs text-fg-muted">by {l.recorded_by_name}</span>}
    </span>
  );
}

const linesText = (b: PayoutBatchSummary) =>
  b.status === "exported" || b.status === "settled"
    ? resultsText(b)
    : `${b.line_count} ${b.line_count === 1 ? "line" : "lines"}`;

function BatchTable({ batches }: { batches: PayoutBatchSummary[] }) {
  const columns: Column<PayoutBatchSummary>[] = [
    {
      key: "period",
      header: "Period",
      sortValue: (b) => b.period_end,
      cell: (b) => (
        <span className="grid gap-0.5">
          <Link
            href={`/w/payouts/${b.id}`}
            className="font-medium underline underline-offset-4 md:whitespace-nowrap"
          >
            {periodText(b.period_start, b.period_end)}
          </Link>
          <span className="text-xs text-fg-muted md:hidden">{linesText(b)}</span>
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      cell: (b) => <StatusPill status={batchTone[b.status]}>{batchLabel[b.status]}</StatusPill>,
    },
    {
      key: "total",
      header: "Total",
      numeric: true,
      sortValue: (b) => b.total_cents,
      cell: (b) => (
        <span className="grid justify-items-end gap-0.5">
          <span className="font-mono whitespace-nowrap tabular">{formatKes(b.total_cents, b.currency)}</span>
          {b.paid_lines > 0 && b.paid_cents !== b.total_cents && (
            <span className="text-xs text-fg-muted">{formatKes(b.paid_cents, b.currency)} paid</span>
          )}
        </span>
      ),
    },
    {
      key: "lines",
      header: "Lines",
      className: "hidden md:table-cell",
      cell: (b) => <span className="text-sm">{linesText(b)}</span>,
    },
    {
      key: "people",
      header: "Prepared and approved by",
      className: "hidden md:table-cell",
      cell: (b) => (
        <span className="grid gap-0.5 text-sm">
          <span>{b.created_by_name}</span>
          <span className="text-fg-muted">{b.approved_by_name ?? "Not approved"}</span>
        </span>
      ),
    },
  ];
  return (
    <Table
      caption="Payout batches"
      hideCaption
      columns={columns}
      rows={batches}
      rowKey={(b) => b.id}
      empty="No batches yet."
    />
  );
}

function NextDate({ initial, onSaved }: { initial: string | null; onSaved: (d: string | null) => void }) {
  const toast = useToast();
  const [date, setDate] = useState(initial ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async (value: string) => {
    setBusy(true);
    setError(null);
    try {
      const r = await api<{ date: string | null }>("PUT", "/v1/payouts/next-date", { date: value });
      setDate(r.date ?? "");
      onSaved(r.date);
      toast({
        tone: "success",
        title: r.date ? `Next payout date set to ${formatDay(r.date)}` : "Next payout date hidden",
      });
    } catch (err) {
      setError(payoutError(asError(err)));
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-labelledby="next-h" className="grid gap-3">
      <h2 id="next-h" className="font-display text-lg font-semibold">
        Next payout date
      </h2>
      <Card className="grid gap-4 p-5">
        <p className="text-sm text-fg-muted">
          {initial
            ? `Contributors see ${formatDay(initial)} as the next payout date on their dashboard.`
            : "No date is shown to contributors. Set one when the next batch is planned."}
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void save(date);
          }}
          className="flex flex-wrap items-end gap-3"
        >
          <Field label="Date shown to contributors" required className="w-full max-w-xs">
            <Input
              type="date"
              required
              min={nairobiToday()}
              value={date}
              onChange={(e) => setDate(e.target.value)}
            />
          </Field>
          <Button type="submit" variant="secondary" disabled={busy || !date || date === initial}>
            Save date
          </Button>
          {initial && (
            <Button type="button" variant="ghost" disabled={busy} onClick={() => void save("")}>
              Hide date
            </Button>
          )}
        </form>
        {error && <Alert tone="danger" title={error} />}
      </Card>
    </section>
  );
}
