"use client";

import { Download } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Card, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { Table, type Column } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { api, ApiError, upload, type Invoice, type Organisation, type Project } from "@/lib/api/client";
import { formatDay } from "@/lib/earnings";
import { errorMessage } from "@/lib/onboarding";
import { invoiceLabel, invoiceTone } from "@/lib/portal";
import { formatKes, kesToCents } from "@/lib/projects";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { PageTitle, useCan } from "./workspace";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

function DownloadLink({ inv }: { inv: Invoice }) {
  return (
    <a
      href={`/api/v1/invoices/${inv.id}/download`}
      download={inv.file_name}
      className="inline-flex items-center gap-1.5 text-sm underline underline-offset-4"
    >
      <Download aria-hidden="true" className="size-4" />
      PDF<span className="sr-only"> of invoice {inv.number}</span>
    </a>
  );
}

/** /w/invoices: Client Admins read their invoices; Ops upload and settle them. */
export function InvoicesPage() {
  const user = useUser();
  const can = useCan();
  const manage = can("invoice.manage");
  const [tick, setTick] = useState(0);
  const list = useLoad(
    () => api<{ invoices: Invoice[] }>("GET", "/v1/invoices").then((r) => r.invoices),
    String(tick),
  );
  const [settling, setSettling] = useState<Invoice | null>(null);
  const reload = () => setTick((t) => t + 1);

  const columns: Column<Invoice>[] = [
    {
      key: "number",
      header: "Invoice",
      sortValue: (i) => i.number,
      cell: (i) => (
        <span className="grid gap-0.5">
          <span className="font-mono">{i.number}</span>
          {i.description && <span className="text-xs text-fg-muted">{i.description}</span>}
          {manage && <span className="text-xs text-fg-muted">{i.org_name}</span>}
          {i.project_name && <span className="text-xs text-fg-muted">{i.project_name}</span>}
          {i.void_reason && <span className="text-xs text-fg-muted">Void: {i.void_reason}</span>}
        </span>
      ),
    },
    {
      key: "issued",
      header: "Issued",
      sortValue: (i) => i.issued_on,
      cell: (i) => <span className="whitespace-nowrap">{formatDay(i.issued_on)}</span>,
    },
    {
      key: "due",
      header: "Due",
      cell: (i) => <span className="whitespace-nowrap">{i.due_on ? formatDay(i.due_on) : "–"}</span>,
    },
    {
      key: "amount",
      header: "Amount",
      numeric: true,
      sortValue: (i) => i.amount_cents,
      cell: (i) => <span className="whitespace-nowrap">{formatKes(i.amount_cents, i.currency)}</span>,
    },
    {
      key: "status",
      header: "Status",
      cell: (i) => (
        <span className="grid justify-items-start gap-0.5">
          <StatusPill status={invoiceTone[i.status]}>{invoiceLabel[i.status]}</StatusPill>
          {i.paid_on && <span className="text-xs text-fg-muted">on {formatDay(i.paid_on)}</span>}
        </span>
      ),
    },
    {
      key: "file",
      header: <span className="sr-only">File</span>,
      cell: (i) => (
        <span className="flex flex-wrap items-center gap-3">
          <DownloadLink inv={i} />
          {manage && i.status !== "void" && (
            <Button variant="ghost" size="sm" onClick={() => setSettling(i)}>
              Update<span className="sr-only"> invoice {i.number}</span>
            </Button>
          )}
        </span>
      ),
    },
  ];

  return (
    <div className="grid gap-10">
      <PageTitle title="Invoices">
        {manage
          ? "Invoices are issued outside MezAI. Upload the PDF here so the client's admins find it with their projects."
          : `Invoices from ELS Software Solutions for ${user.kind === "client" ? "your organisation" : "this organisation"}. Payment is arranged outside MezAI, as set out in your agreement.`}
      </PageTitle>
      {manage && <UploadForm onDone={reload} />}
      <section aria-labelledby="invoices-h" className="grid gap-3">
        <h2 id="invoices-h" className={manage ? "font-display text-lg font-semibold" : "sr-only"}>
          {manage ? "All invoices" : "Invoices"}
        </h2>
        {list.status === "loading" && <Skeleton className="h-40 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" &&
          (list.data.length === 0 ? (
            <EmptyState title="No invoices yet">
              {manage
                ? "Upload the first invoice above."
                : "Invoices appear here when the MezAI team adds them."}
            </EmptyState>
          ) : (
            <Table
              caption="Invoices"
              hideCaption
              columns={columns}
              rows={list.data}
              rowKey={(i) => i.id}
              empty="No invoices yet."
            />
          ))}
      </section>
      {settling && (
        <SettleDialog
          inv={settling}
          onClose={() => setSettling(null)}
          onDone={() => {
            setSettling(null);
            reload();
          }}
        />
      )}
    </div>
  );
}

const today = () => new Date(Date.now() + 3 * 3600_000).toISOString().slice(0, 10);

function UploadForm({ onDone }: { onDone: () => void }) {
  const toast = useToast();
  const orgs = useLoad(() =>
    api<{ organisations: Organisation[] }>("GET", "/v1/organisations").then((r) => r.organisations),
  );
  const projects = useLoad(() => api<{ projects: Project[] }>("GET", "/v1/projects").then((r) => r.projects));
  const [f, setF] = useState({
    org: "",
    project: "",
    number: "",
    issued: today(),
    due: "",
    amount: "",
    note: "",
  });
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const set = (k: keyof typeof f) => (e: { target: { value: string } }) =>
    setF((x) => ({ ...x, [k]: e.target.value }));
  const org = f.org || (orgs.status === "ready" ? (orgs.data[0]?.id ?? "") : "");
  const orgProjects = projects.status === "ready" ? projects.data.filter((p) => p.org_id === org) : [];

  const submit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const cents = kesToCents(f.amount);
    if (cents == null) return setError("Enter the amount in KES, for example 12500 or 12,500.00.");
    if (!file) return setError("Attach the invoice PDF.");
    const form = new FormData();
    form.set("org_id", org);
    if (f.project) form.set("project_id", f.project);
    form.set("number", f.number.trim());
    form.set("issued_on", f.issued);
    if (f.due) form.set("due_on", f.due);
    form.set("amount_cents", String(cents));
    form.set("description", f.note.trim());
    form.set("file", file);
    setBusy(true);
    setError(null);
    try {
      await upload("/v1/invoices", form);
      toast({ tone: "success", title: `Invoice ${f.number.trim()} added` });
      setF((x) => ({ ...x, number: "", due: "", amount: "", note: "", project: "" }));
      setFile(null);
      (e.target as HTMLFormElement).reset();
      onDone();
    } catch (err) {
      const a = asError(err);
      setError(
        a.code === "conflict"
          ? "This organisation already has an invoice with that number."
          : a.code === "invalid_input"
            ? "Check the fields: the number may use letters, digits and / . _ -, the due day cannot be before the issue day, and the file must be a PDF."
            : errorMessage(a),
      );
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-labelledby="upload-h" className="grid gap-3">
      <h2 id="upload-h" className="font-display text-lg font-semibold">
        Add an invoice
      </h2>
      <Card className="p-5">
        <form onSubmit={submit} className="grid gap-4 md:grid-cols-2">
          <Field label="Organisation" required>
            <Select value={org} onChange={(e) => setF((x) => ({ ...x, org: e.target.value, project: "" }))}>
              {orgs.status === "ready" &&
                orgs.data.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
            </Select>
          </Field>
          <Field label="Project" hint="Optional">
            <Select value={f.project} onChange={set("project")}>
              <option value="">Not for one project</option>
              {orgProjects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Invoice number" required>
            <Input required maxLength={60} value={f.number} onChange={set("number")} />
          </Field>
          <Field label="Amount (KES)" required>
            <Input required inputMode="decimal" value={f.amount} onChange={set("amount")} />
          </Field>
          <Field label="Issued on" required>
            <Input type="date" required value={f.issued} onChange={set("issued")} />
          </Field>
          <Field label="Due on" hint="Optional">
            <Input type="date" min={f.issued} value={f.due} onChange={set("due")} />
          </Field>
          <Field label="Description" hint="Optional; the client sees it" className="md:col-span-2">
            <Textarea rows={2} maxLength={500} value={f.note} onChange={set("note")} />
          </Field>
          <Field label="Invoice PDF" hint="Up to 5 MB" required className="md:col-span-2">
            <Input
              type="file"
              accept="application/pdf,.pdf"
              required
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </Field>
          {error && <Alert tone="danger" title={error} className="md:col-span-2" />}
          <div className="md:col-span-2">
            <Button type="submit" disabled={busy || !org}>
              {busy ? "Uploading…" : "Add invoice"}
            </Button>
          </div>
        </form>
      </Card>
    </section>
  );
}

function SettleDialog({ inv, onClose, onDone }: { inv: Invoice; onClose: () => void; onDone: () => void }) {
  const toast = useToast();
  const [status, setStatus] = useState<"paid" | "void">(inv.status === "issued" ? "paid" : "void");
  const [paidOn, setPaidOn] = useState(today());
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      await api(
        "POST",
        `/v1/invoices/${inv.id}/status`,
        status === "paid" ? { status, paid_on: paidOn } : { status, reason },
      );
      toast({
        tone: "success",
        title: `Invoice ${inv.number} marked ${status === "paid" ? "paid" : "void"}`,
      });
      onDone();
    } catch (err) {
      setError(errorMessage(asError(err)));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      onClose={onClose}
      title={`Update invoice ${inv.number}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={save} disabled={busy || (status === "void" && !reason.trim())}>
            {busy ? "Saving…" : "Save"}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <Field label="New status">
          <Select value={status} onChange={(e) => setStatus(e.target.value as "paid" | "void")}>
            {inv.status === "issued" && <option value="paid">Paid</option>}
            <option value="void">Void</option>
          </Select>
        </Field>
        {status === "paid" ? (
          <Field label="Paid on" required>
            <Input type="date" value={paidOn} max={today()} onChange={(e) => setPaidOn(e.target.value)} />
          </Field>
        ) : (
          <Field
            label="Reason"
            hint="The client sees this, and a void invoice cannot be changed again."
            required
          >
            <Textarea rows={3} maxLength={500} value={reason} onChange={(e) => setReason(e.target.value)} />
          </Field>
        )}
        {error && <Alert tone="danger" title={error} />}
      </div>
    </Dialog>
  );
}
