"use client";

import { Check, Download } from "lucide-react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Breadcrumb, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { ChoiceGroup, Radio } from "@/components/ui/choice";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { api, ApiError } from "@/lib/api/client";
import {
  b2cMode,
  batchLabel,
  stillPaying,
  type B2CStatus,
  type BatchTransactions,
  batchTone,
  nextStep,
  normaliseReceipt,
  payoutError,
  periodText,
  resultsText,
  steps,
  uncovered,
  type PayoutBatch,
  type PayoutLine,
} from "@/lib/payouts";
import { formatKes } from "@/lib/projects";
import { useUser } from "@/lib/session";
import { cn } from "@/lib/cn";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { Figure, LinesTable } from "./payouts";
import { useCan } from "./workspace";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

/** Seconds between refreshes while M-Pesa results are still arriving. */
const REFRESH_S = 10;

type Loaded = { batch: PayoutBatch; pay: BatchTransactions };

/** /w/payouts/[id]: one batch, the next step for the person looking, and its lines. */
export function PayoutBatchPage() {
  const { id } = useParams<{ id: string }>();
  const d = useLoad(async (): Promise<Loaded> => {
    const [batch, pay] = await Promise.all([
      api<PayoutBatch>("GET", `/v1/payouts/batches/${id}`),
      api<BatchTransactions>("GET", `/v1/payouts/batches/${id}/transactions`),
    ]);
    return { batch, pay };
  }, id);
  const paying =
    d.status === "ready" && (d.data.batch.status === "disbursing" || stillPaying(d.data.pay.transactions));
  const reload = d.reload;
  // M-Pesa results arrive by themselves: refresh while the tab is visible.
  useEffect(() => {
    if (!paying) return;
    const t = window.setInterval(() => {
      if (document.visibilityState === "visible") reload();
    }, REFRESH_S * 1000);
    return () => window.clearInterval(t);
  }, [paying, reload]);

  if (d.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (d.status === "error") return <Alert tone="danger" title={payoutError(d.error)} />;
  const { batch, pay } = d.data;
  return (
    <BatchView
      batch={batch}
      pay={pay}
      onChange={(nb) => {
        d.set({ batch: nb, pay });
        reload(); // a disburse or stop changes the payments too
      }}
    />
  );
}

function BatchView({
  batch: b,
  pay,
  onChange,
}: {
  batch: PayoutBatch;
  pay: BatchTransactions;
  onChange: (b: PayoutBatch) => void;
}) {
  const user = useUser();
  const can = useCan();
  const who = { userId: user.id, canCreate: can("payout.create"), canApprove: can("payout.approve") };
  const step = nextStep(b, who);
  const [recording, setRecording] = useState<PayoutLine | null>(null);
  const [stopping, setStopping] = useState<PayoutLine | null>(null);
  const payments = new Map(pay.transactions.map((t) => [t.line_id, t]));
  const b2cBatch = b.disbursed_at !== null || pay.transactions.length > 0;
  const short = uncovered(b.lines);
  const title = periodText(b.period_start, b.period_end);

  return (
    <div className="grid gap-8">
      <div className="grid gap-3">
        <Breadcrumb items={[{ label: "Payouts", href: "/w/payouts" }, { label: title }]} />
        <header className="flex flex-wrap items-start justify-between gap-3">
          <div className="grid gap-1.5">
            <h1 className="font-display text-2xl font-semibold tracking-[-0.01em]">Payout batch, {title}</h1>
            <p className="text-sm text-fg-muted">
              Prepared by {b.created_by_name} on {shortDate(b.created_at)}
              {b.approved_by_name && b.approved_at && (
                <>
                  {" "}
                  · approved by {b.approved_by_name} on {shortDate(b.approved_at)}
                </>
              )}
            </p>
          </div>
          <StatusPill status={batchTone[b.status]}>{batchLabel[b.status]}</StatusPill>
        </header>
      </div>

      <Steps batch={b} />

      <Card className="p-5">
        <dl className="grid gap-4 sm:grid-cols-3">
          <Figure label="Total" value={formatKes(b.total_cents, b.currency)} mono />
          <Figure label="Contributors" value={`${b.line_count}`} />
          <Figure label="Paid so far" value={formatKes(b.paid_cents, b.currency)} mono />
        </dl>
      </Card>

      {short.length > 0 && b.status !== "exported" && b.status !== "disbursing" && (
        <Alert tone="danger" title="A balance is now less than its line">
          {short.length === 1 ? "One contributor's" : `${short.length} contributors'`} balance dropped after a
          correction, so this batch cannot be approved or exported. Cancel it and create a new one.
        </Alert>
      )}

      <NextAction batch={b} b2c={pay.b2c} step={step} onChange={onChange} />

      <section aria-labelledby="lines-h" className="grid gap-3">
        <h2 id="lines-h" className="font-display text-lg font-semibold">
          Lines
        </h2>
        <p className="text-sm text-fg-muted">
          {b2cBatch
            ? "M-Pesa numbers are masked here. M-Pesa's registered names are compared with the profile name and never stored."
            : "M-Pesa numbers are masked here. The full numbers are only in the exported file."}
        </p>
        <LinesTable
          caption="Lines in this batch"
          lines={b.lines}
          showBalance={b.status !== "settled" && b.status !== "cancelled"}
          showResult={b.status === "exported" || b.status === "disbursing" || b.status === "settled"}
          payments={payments}
          action={
            step === "record"
              ? (l) =>
                  l.status === "pending" && (
                    <Button size="sm" variant="secondary" onClick={() => setRecording(l)}>
                      Record result<span className="sr-only"> for {l.public_code}</span>
                    </Button>
                  )
              : step === "b2c"
                ? (l) =>
                    l.status === "pending" &&
                    payments.get(l.id)?.status === "queued" && (
                      <Button size="sm" variant="ghost" onClick={() => setStopping(l)}>
                        Stop<span className="sr-only"> payment to {l.public_code}</span>
                      </Button>
                    )
                : undefined
          }
        />
      </section>

      {stopping && (
        <StopDialog
          line={stopping}
          onClose={() => setStopping(null)}
          onDone={(nb) => {
            setStopping(null);
            onChange(nb);
          }}
        />
      )}
      {recording && (
        <ResultDialog
          line={recording}
          onClose={() => setRecording(null)}
          onDone={(nb) => {
            setRecording(null);
            onChange(nb);
          }}
        />
      )}
    </div>
  );
}

function Steps({ batch: b }: { batch: PayoutBatch }) {
  const list = steps(b.status);
  if (list.length === 0) return null;
  return (
    <ol className="grid grid-cols-5 gap-2" aria-label="Batch steps">
      {list.map((s, i) => (
        <li
          key={s.label}
          className="grid content-start gap-2"
          aria-current={s.state === "current" ? "step" : undefined}
        >
          <span
            className={cn(
              "h-1 rounded-full",
              s.state === "done" ? "bg-fg" : s.state === "current" ? "bg-lime" : "bg-border",
            )}
          />
          <span
            className={cn(
              "flex items-center gap-1 text-xs sm:text-sm",
              s.state === "todo" ? "text-fg-muted" : "font-semibold text-fg",
            )}
          >
            {s.state === "done" && <Check aria-hidden="true" className="hidden size-3.5 sm:block" />}
            <span>
              <span className="sr-only">
                Step {i + 1}, {s.state === "done" ? "done" : s.state === "current" ? "current" : "to do"}
                :{" "}
              </span>
              {s.label}
            </span>
          </span>
        </li>
      ))}
    </ol>
  );
}

type Act = "submit" | "approve" | "return" | "cancel" | "export" | "disburse";

function NextAction({
  batch: b,
  b2c,
  step,
  onChange,
}: {
  batch: PayoutBatch;
  b2c: B2CStatus;
  step: ReturnType<typeof nextStep>;
  onChange: (b: PayoutBatch) => void;
}) {
  const toast = useToast();
  const can = useCan();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [asking, setAsking] = useState<"return" | "cancel" | null>(null);
  const [confirmPay, setConfirmPay] = useState(false);
  const mode = b2cMode(b2c);

  const run = async (act: Act, reason?: string) => {
    setBusy(true);
    setError(null);
    try {
      const nb = await api<PayoutBatch>(
        "POST",
        `/v1/payouts/batches/${b.id}/${act}`,
        reason === undefined ? undefined : { reason },
      );
      const done: Record<Act, string> = {
        submit: "Sent for approval",
        approve: "Batch approved",
        return: "Sent back to the preparer",
        cancel: "Batch cancelled",
        export: "Payment file ready",
        disburse: "Payments are going out through M-Pesa",
      };
      toast({ tone: "success", title: done[act] });
      setAsking(null);
      setConfirmPay(false);
      onChange(nb);
    } catch (err) {
      setError(payoutError(asError(err)));
    } finally {
      setBusy(false);
    }
  };

  const canCancel = can("payout.create") && (b.status === "draft" || b.status === "pending_approval");
  const cancelButton = canCancel && (
    <Button variant="ghost" disabled={busy} onClick={() => setAsking("cancel")}>
      Cancel batch
    </Button>
  );

  let body: React.ReactNode = null;
  switch (step) {
    case "submit":
      body = (
        <Panel title="Check the lines, then send for approval">
          {b.returned_note && (
            <Alert tone="warning" title="Sent back by the approver">
              {b.returned_note}
            </Alert>
          )}
          <p className="text-sm text-fg-muted">
            A second person with payout approval checks the batch before the file can be exported.
          </p>
          <div className="flex flex-wrap gap-3">
            <Button disabled={busy} onClick={() => void run("submit")}>
              Send for approval
            </Button>
            {cancelButton}
          </div>
        </Panel>
      );
      break;
    case "approve":
      body = (
        <Panel title="Your approval is needed">
          <p className="text-sm text-fg-muted">
            {b.created_by_name} prepared this batch. Check each line against the contributor&apos;s balance,
            then approve it or send it back with a note.
          </p>
          <div className="flex flex-wrap gap-3">
            <Button disabled={busy} onClick={() => void run("approve")}>
              Approve batch
            </Button>
            <Button variant="secondary" disabled={busy} onClick={() => setAsking("return")}>
              Send back
            </Button>
          </div>
        </Panel>
      );
      break;
    case "own_batch":
    case "wait_approval":
      body = (
        <Panel title="Waiting for approval">
          <p className="text-sm text-fg-muted">
            {step === "own_batch"
              ? "You prepared this batch, so another person with payout approval must approve it."
              : "A person with payout approval who did not prepare this batch must approve it."}
          </p>
          {cancelButton && <div>{cancelButton}</div>}
        </Panel>
      );
      break;
    case "export":
      body = b2c.enabled ? (
        <Panel title="Pay the batch">
          <ModeNote b2c={b2c} />
          <p className="text-sm text-fg-muted">
            Pay every line straight from MezAI through M-Pesa, or export the bulk payment file and pay it in
            the business portal. A batch is paid one way only.
          </p>
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid content-start gap-2 rounded-card border border-border p-4">
              <h3 className="font-semibold">Pay through M-Pesa</h3>
              <p className="text-sm text-fg-muted">
                One payment per line, sent once and never twice. Results come back by themselves and each line
                is marked paid or failed.
              </p>
              <div>
                <Button disabled={busy} onClick={() => setConfirmPay(true)}>
                  Pay through M-Pesa
                </Button>
              </div>
            </div>
            <div className="grid content-start gap-2 rounded-card border border-border p-4">
              <h3 className="font-semibold">Use the payment file</h3>
              <p className="text-sm text-fg-muted">
                The file holds the full phone numbers. You record each line&apos;s result here afterwards.
              </p>
              {b.format_confirmed === false && (
                <p className="text-sm font-semibold text-warning">File columns not yet confirmed (DG-03).</p>
              )}
              <div>
                <Button variant="secondary" disabled={busy} onClick={() => void run("export")}>
                  Export payment file
                </Button>
              </div>
            </div>
          </div>
        </Panel>
      ) : (
        <Panel title="Export the payment file">
          {b.format_confirmed === false && <FormatWarning />}
          <p className="text-sm text-fg-muted">
            The file holds the full phone numbers. Download it only on a trusted computer, upload it to the
            M-Pesa business portal, and delete your copy once the payment is done.
          </p>
          <div>
            <Button disabled={busy} onClick={() => void run("export")}>
              Export payment file
            </Button>
          </div>
        </Panel>
      );
      break;
    case "wait_export":
      body = (
        <Panel title="Approved">
          <p className="text-sm text-fg-muted">
            The operations team exports the payment file and pays the batch next.
          </p>
        </Panel>
      );
      break;
    case "b2c":
    case "watch":
      if (b.status === "disbursing") {
        body = (
          <Panel title="Paying through M-Pesa">
            <ModeNote b2c={b2c} />
            <p className="text-sm text-fg-muted" aria-live="polite">
              {resultsText(b)}. Results arrive by themselves and this page refreshes every {REFRESH_S}{" "}
              seconds. A payment M-Pesa did not answer clearly is never sent again: it is checked until M-Pesa
              says whether it was paid. A failed line&apos;s money stays owed and joins the next batch.
            </p>
            {step === "b2c" && (
              <p className="text-sm text-fg-muted">
                A payment M-Pesa has not accepted yet can be stopped from its line.
              </p>
            )}
            <div>
              <Link
                href="/w/payouts/reconciliation"
                className="text-sm font-medium underline underline-offset-4"
              >
                M-Pesa reconciliation
              </Link>
            </div>
          </Panel>
        );
        break;
      }
    // falls through: an exported batch this person can only watch
    case "record":
      body = (
        <Panel title={step === "record" ? "Pay in M-Pesa, then record each result" : "Being paid"}>
          {b.format_confirmed === false && <FormatWarning />}
          <p className="text-sm text-fg-muted">
            {resultsText(b)}.{" "}
            {step === "record" &&
              "Record a line paid with its M-Pesa receipt, or failed with a reason the contributor will see. A failed line's money stays owed and joins the next batch. A recorded result cannot be changed."}
          </p>
          {b.export_file_name && (
            <div>
              <a
                href={`/api/v1/payouts/batches/${b.id}/file`}
                download={b.export_file_name}
                className="inline-flex items-center gap-1.5 text-sm font-medium underline underline-offset-4"
              >
                <Download aria-hidden="true" className="size-4" />
                Download {b.export_file_name}
              </a>
            </div>
          )}
        </Panel>
      );
      break;
    default:
      if (b.status === "settled" && b.settled_at)
        body = (
          <Alert tone="success" title={`Settled on ${shortDate(b.settled_at)}`}>
            Every line has a result. {resultsText(b)}.
          </Alert>
        );
      if (b.status === "cancelled")
        body = (
          <Alert tone="info" title={`Cancelled${b.cancelled_by_name ? ` by ${b.cancelled_by_name}` : ""}`}>
            {b.cancel_reason} The money stays owed and can go into a new batch.
          </Alert>
        );
  }

  return (
    <>
      {body}
      {error && <Alert tone="danger" title={error} />}
      {confirmPay && (
        <Dialog
          open
          onClose={() => setConfirmPay(false)}
          title="Pay this batch through M-Pesa?"
          description={`${b.line_count} ${b.line_count === 1 ? "payment" : "payments"} totalling ${formatKes(b.total_cents, b.currency)} will be sent. Once sent, a payment cannot be called back.`}
          footer={
            <>
              <Button variant="secondary" onClick={() => setConfirmPay(false)}>
                Not now
              </Button>
              <Button disabled={busy} onClick={() => void run("disburse")}>
                {busy ? "Starting…" : mode.real ? "Send real payments" : "Send test payments"}
              </Button>
            </>
          }
        >
          <div className="grid gap-4">
            <Alert tone={mode.real ? "warning" : "info"} title={mode.label}>
              {mode.note}
            </Alert>
            {error && <Alert tone="danger" title={error} />}
          </div>
        </Dialog>
      )}
      {asking && (
        <ReasonDialog
          kind={asking}
          busy={busy}
          error={error}
          onClose={() => {
            setAsking(null);
            setError(null);
          }}
          onConfirm={(reason) => void run(asking, reason)}
        />
      )}
    </>
  );
}

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section
      aria-labelledby="next-step-h"
      className="grid gap-3 rounded-card border border-border-strong p-5"
    >
      <h2 id="next-step-h" className="font-display text-lg font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

function ModeNote({ b2c }: { b2c: B2CStatus }) {
  const mode = b2cMode(b2c);
  if (mode.real) return null;
  return (
    <Alert tone="info" title={`${mode.label}: no money moves`}>
      {mode.note}
    </Alert>
  );
}

function StopDialog({
  line,
  onClose,
  onDone,
}: {
  line: PayoutLine;
  onClose: () => void;
  onDone: (b: PayoutBatch) => void;
}) {
  const toast = useToast();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const stop = async () => {
    setBusy(true);
    setError(null);
    try {
      const nb = await api<PayoutBatch>("POST", `/v1/payouts/lines/${line.id}/stop`, {
        reason: reason.trim(),
      });
      toast({ tone: "success", title: `Payment to ${line.public_code} stopped` });
      onDone(nb);
    } catch (err) {
      const e = asError(err);
      setError(
        e.code === "wrong_state"
          ? "M-Pesa has already taken this payment, so it cannot be stopped. Wait for its result."
          : payoutError(e),
      );
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={`Stop the payment to ${line.public_code}`}
      description={`${formatKes(line.amount_cents, line.currency)} has not been accepted by M-Pesa yet. Stopping it marks the line failed; nothing is paid and the money stays owed for the next batch.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Keep trying
          </Button>
          <Button variant="danger" disabled={busy || !reason.trim()} onClick={() => void stop()}>
            {busy ? "Stopping…" : "Stop payment"}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <Field label="Reason" hint="The contributor sees this." required>
          <Textarea rows={3} maxLength={500} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>
        {error && <Alert tone="danger" title={error} />}
      </div>
    </Dialog>
  );
}

function FormatWarning() {
  return (
    <Alert tone="warning" title="File columns not yet confirmed">
      The column layout has not been checked against the M-Pesa bulk payment template (DG-03). Compare the
      file with the portal&apos;s template before uploading it.
    </Alert>
  );
}

function ReasonDialog({
  kind,
  busy,
  error,
  onClose,
  onConfirm,
}: {
  kind: "return" | "cancel";
  busy: boolean;
  error: string | null;
  onClose: () => void;
  onConfirm: (reason: string) => void;
}) {
  const [reason, setReason] = useState("");
  return (
    <Dialog
      open
      onClose={onClose}
      title={kind === "return" ? "Send the batch back" : "Cancel the batch"}
      description={
        kind === "return"
          ? "The batch goes back to draft. The preparer sees your note."
          : "The batch is kept with your reason, and nobody in it is paid. Their money stays owed."
      }
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Keep it
          </Button>
          <Button
            variant={kind === "cancel" ? "danger" : "primary"}
            disabled={busy || !reason.trim()}
            onClick={() => onConfirm(reason.trim())}
          >
            {kind === "return" ? "Send back" : "Cancel batch"}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <Field label={kind === "return" ? "Note for the preparer" : "Reason"} required>
          <Textarea rows={3} maxLength={500} value={reason} onChange={(e) => setReason(e.target.value)} />
        </Field>
        {error && <Alert tone="danger" title={error} />}
      </div>
    </Dialog>
  );
}

function ResultDialog({
  line,
  onClose,
  onDone,
}: {
  line: PayoutLine;
  onClose: () => void;
  onDone: (b: PayoutBatch) => void;
}) {
  const toast = useToast();
  const [status, setStatus] = useState<"paid" | "failed">("paid");
  const [receipt, setReceipt] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const r = normaliseReceipt(receipt);

  const save = async () => {
    if (status === "paid" && !r.ok)
      return setError("Enter the M-Pesa receipt: 8 to 12 letters and digits, for example QK41ABCD2E.");
    setBusy(true);
    setError(null);
    try {
      const nb = await api<PayoutBatch>(
        "POST",
        `/v1/payouts/lines/${line.id}/result`,
        status === "paid" ? { status, receipt: r.receipt } : { status, reason: reason.trim() },
      );
      toast({ tone: "success", title: `${line.public_code} recorded ${status}` });
      onDone(nb);
    } catch (err) {
      const e = asError(err);
      setError(
        e.code === "wrong_state"
          ? "This line already has a different result, or the batch moved on. Reload the page."
          : payoutError(e),
      );
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={`Record the result for ${line.public_code}`}
      description={`${formatKes(line.amount_cents, line.currency)} to ${line.phone_masked}. A recorded result cannot be changed.`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            onClick={() => void save()}
            disabled={busy || (status === "paid" ? !receipt.trim() : !reason.trim())}
          >
            {busy ? "Saving…" : status === "paid" ? "Record paid" : "Record failed"}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <ChoiceGroup legend="Result">
          {(["paid", "failed"] as const).map((s) => (
            <Radio
              key={s}
              name="result"
              value={s}
              checked={status === s}
              onChange={() => {
                setStatus(s);
                setError(null);
              }}
              label={s === "paid" ? "Paid in M-Pesa" : "Failed"}
            />
          ))}
        </ChoiceGroup>
        {status === "paid" ? (
          <Field label="M-Pesa receipt" hint="From the portal's results, for example QK41ABCD2E" required>
            <Input
              autoComplete="off"
              spellCheck={false}
              maxLength={20}
              className="font-mono uppercase"
              value={receipt}
              onChange={(e) => setReceipt(e.target.value)}
            />
          </Field>
        ) : (
          <Field label="Reason" hint="The contributor sees this. Their money stays owed." required>
            <Textarea rows={3} maxLength={500} value={reason} onChange={(e) => setReason(e.target.value)} />
          </Field>
        )}
        {error && <Alert tone="danger" title={error} />}
      </div>
    </Dialog>
  );
}
