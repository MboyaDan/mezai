"use client";

import { Check, Download } from "lucide-react";
import { useParams } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Breadcrumb, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { ChoiceGroup, Radio } from "@/components/ui/choice";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { api, ApiError } from "@/lib/api/client";
import {
  batchLabel,
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

/** /w/payouts/[id]: one batch, the next step for the person looking, and its lines. */
export function PayoutBatchPage() {
  const { id } = useParams<{ id: string }>();
  const b = useLoad(() => api<PayoutBatch>("GET", `/v1/payouts/batches/${id}`), id);
  if (b.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (b.status === "error") return <Alert tone="danger" title={payoutError(b.error)} />;
  return <BatchView batch={b.data} onChange={b.set} />;
}

function BatchView({ batch: b, onChange }: { batch: PayoutBatch; onChange: (b: PayoutBatch) => void }) {
  const user = useUser();
  const can = useCan();
  const who = { userId: user.id, canCreate: can("payout.create"), canApprove: can("payout.approve") };
  const step = nextStep(b, who);
  const [recording, setRecording] = useState<PayoutLine | null>(null);
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

      {short.length > 0 && b.status !== "exported" && (
        <Alert tone="danger" title="A balance is now less than its line">
          {short.length === 1 ? "One contributor's" : `${short.length} contributors'`} balance dropped after a
          correction, so this batch cannot be approved or exported. Cancel it and create a new one.
        </Alert>
      )}

      <NextAction batch={b} step={step} onChange={onChange} />

      <section aria-labelledby="lines-h" className="grid gap-3">
        <h2 id="lines-h" className="font-display text-lg font-semibold">
          Lines
        </h2>
        <p className="text-sm text-fg-muted">
          M-Pesa numbers are masked here. The full numbers are only in the exported file.
        </p>
        <LinesTable
          caption="Lines in this batch"
          lines={b.lines}
          showBalance={b.status !== "settled" && b.status !== "cancelled"}
          showResult={b.status === "exported" || b.status === "settled"}
          action={
            step === "record"
              ? (l) =>
                  l.status === "pending" && (
                    <Button size="sm" variant="secondary" onClick={() => setRecording(l)}>
                      Record result<span className="sr-only"> for {l.public_code}</span>
                    </Button>
                  )
              : undefined
          }
        />
      </section>

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

type Act = "submit" | "approve" | "return" | "cancel" | "export";

function NextAction({
  batch: b,
  step,
  onChange,
}: {
  batch: PayoutBatch;
  step: ReturnType<typeof nextStep>;
  onChange: (b: PayoutBatch) => void;
}) {
  const toast = useToast();
  const can = useCan();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [asking, setAsking] = useState<"return" | "cancel" | null>(null);

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
      };
      toast({ tone: "success", title: done[act] });
      setAsking(null);
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
      body = (
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
    case "record":
    case "watch":
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
