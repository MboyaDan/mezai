"use client";

import Link from "next/link";
import { useParams, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/choice";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Breadcrumb, Card, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Select, Textarea } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type Organisation } from "@/lib/api/client";
import {
  buttonMoves,
  change,
  enquiryStatuses,
  moveLabel,
  openStatuses,
  projectTypeLabel,
  statusText,
  timelineLabel,
  volumeLabel,
  type EnquiryDetail as Detail,
  type EnquiryNote,
  type EnquiryStatus,
  type EnquirySummary,
} from "@/lib/enquiry-inbox";
import { errorMessage } from "@/lib/onboarding";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { PageTitle, useCan } from "./workspace";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

function EnquiryStatusPill({ status }: { status: EnquiryStatus }) {
  return <StatusPill status={statusText[status].tone}>{statusText[status].label}</StatusPill>;
}

type Filter = "" | "all" | EnquiryStatus;

const filters: Filter[] = ["", "all", ...enquiryStatuses];

export function Enquiries() {
  const fromUrl = useSearchParams().get("status") ?? "";
  const [status, setStatus] = useState<Filter>(
    filters.includes(fromUrl as Filter) ? (fromUrl as Filter) : "",
  );
  const [mine, setMine] = useState(false);
  const query = new URLSearchParams();
  if (status) query.set("status", status);
  if (mine) query.set("assigned", "me");
  const qs = query.toString();
  const list = useLoad(
    () =>
      api<{ enquiries: EnquirySummary[]; counts: Record<string, number> }>(
        "GET",
        `/v1/enquiries${qs ? `?${qs}` : ""}`,
      ),
    qs,
  );
  const counts = list.status === "ready" ? list.data.counts : null;
  const open = counts ? openStatuses.reduce((n, s) => n + (counts[s] ?? 0), 0) : null;
  const n = (k: string) => (counts ? ` (${counts[k] ?? 0})` : "");

  return (
    <>
      <PageTitle title="Enquiries">
        Project enquiries from the contact form. Assign each one, keep notes on the conversation, and close it
        as won, lost or spam.
      </PageTitle>
      <div className="grid gap-4">
        <div className="flex flex-wrap items-end gap-4">
          <Field label="Status" className="w-56">
            <Select value={status} onChange={(e) => setStatus(e.target.value as Filter)}>
              <option value="">Open{open == null ? "" : ` (${open})`}</option>
              {enquiryStatuses.map((s) => (
                <option key={s} value={s}>
                  {statusText[s].label}
                  {n(s)}
                </option>
              ))}
              <option value="all">All</option>
            </Select>
          </Field>
          <Checkbox label="Assigned to me" checked={mine} onChange={(e) => setMine(e.target.checked)} />
        </div>
        {list.status === "loading" && <Skeleton className="h-40 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" && (
          <Table
            caption="Enquiries, newest first"
            hideCaption
            rows={list.data.enquiries}
            rowKey={(r) => r.id}
            empty={
              <EmptyState title={status || mine ? "No enquiries match" : "No open enquiries"}>
                New enquiries from the contact page appear here and are emailed to the operations inbox.
              </EmptyState>
            }
            columns={[
              {
                key: "org",
                header: "From",
                cell: (r) => (
                  <span className="grid gap-0.5">
                    <Link href={`/w/enquiries/${r.id}`} className="font-medium underline underline-offset-4">
                      {r.organisation}
                    </Link>
                    <span className="text-fg-muted">{r.name}</span>
                  </span>
                ),
                sortValue: (r) => r.organisation.toLowerCase(),
              },
              { key: "type", header: "Interested in", cell: (r) => projectTypeLabel(r.project_type) },
              { key: "status", header: "Status", cell: (r) => <EnquiryStatusPill status={r.status} /> },
              {
                key: "who",
                header: "Assigned to",
                cell: (r) => r.assigned_name ?? <span className="text-fg-muted">Nobody</span>,
                sortValue: (r) => r.assigned_name ?? "",
              },
              { key: "notes", header: "Notes", cell: (r) => r.note_count, sortValue: (r) => r.note_count },
              {
                key: "at",
                header: "Received",
                cell: (r) => shortDate(r.created_at),
                sortValue: (r) => r.created_at,
              },
            ]}
          />
        )}
        {list.status === "ready" && list.data.enquiries.length === 200 && (
          <p className="text-sm text-fg-muted">
            Showing the newest 200. Narrow the status to see older ones.
          </p>
        )}
      </div>
    </>
  );
}

function WonDialog({
  open,
  onClose,
  onConfirm,
  busy,
  current,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: (orgId: string | null) => void;
  busy: boolean;
  current: string | null;
}) {
  const can = useCan();
  const orgs = useLoad(
    () =>
      can("org.manage")
        ? api<{ organisations: Organisation[] }>("GET", "/v1/organisations").then((r) =>
            r.organisations.filter((o) => o.status === "active"),
          )
        : Promise.resolve([] as Organisation[]),
    open ? "open" : "closed",
  );
  const [org, setOrg] = useState(current ?? "");
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title="Mark this enquiry won?"
      description="Won is final. You can link the client's organisation now or later."
      footer={
        <>
          <Button variant="secondary" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <Button size="sm" onClick={() => onConfirm(org || null)} disabled={busy}>
            Mark won
          </Button>
        </>
      }
    >
      <OrgPicker orgs={orgs.status === "ready" ? orgs.data : null} value={org} onChange={setOrg} />
    </Dialog>
  );
}

function OrgPicker({
  orgs,
  value,
  onChange,
}: {
  orgs: Organisation[] | null;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <Field
      label="Organisation"
      hint={
        <>
          A new client is added on the{" "}
          <Link href="/w/organisations" className="underline underline-offset-4">
            Organisations
          </Link>{" "}
          page first.
        </>
      }
    >
      <Select value={value} onChange={(e) => onChange(e.target.value)} disabled={!orgs}>
        <option value="">Not linked</option>
        {orgs?.map((o) => (
          <option key={o.id} value={o.id}>
            {o.name}
          </option>
        ))}
      </Select>
    </Field>
  );
}

function Notes({ enquiry, onAdded }: { enquiry: Detail; onAdded: (n: EnquiryNote) => void }) {
  const [body, setBody] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function add(e: React.FormEvent) {
    e.preventDefault();
    if (!body.trim()) return setError("Write a note.");
    setBusy(true);
    setError(null);
    try {
      onAdded(await api<EnquiryNote>("POST", `/v1/enquiries/${enquiry.id}/notes`, { body }));
      setBody("");
    } catch (x) {
      setError(errorMessage(asError(x)));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card className="grid gap-4 p-5">
      <h2 className="font-display text-base font-semibold">Notes</h2>
      {enquiry.notes.length === 0 ? (
        <p className="text-sm text-fg-muted">No notes yet.</p>
      ) : (
        <ol className="grid gap-3">
          {enquiry.notes.map((n) => (
            <li key={n.id} className="grid gap-1 border-l-2 border-border pl-3 text-sm">
              <span className="text-fg-muted">
                {n.author_name},{" "}
                {new Date(n.created_at).toLocaleString("en-KE", { dateStyle: "medium", timeStyle: "short" })}
              </span>
              <span className="leading-relaxed whitespace-pre-wrap">{n.body}</span>
            </li>
          ))}
        </ol>
      )}
      <form onSubmit={add} noValidate className="grid gap-3">
        <Field
          label="Add a note"
          hint="Internal: only staff who work enquiries see notes. Notes cannot be edited."
          error={error ?? undefined}
        >
          <Textarea rows={3} value={body} maxLength={5000} onChange={(e) => setBody(e.target.value)} />
        </Field>
        <div>
          <Button type="submit" size="sm" disabled={busy}>
            {busy ? "Adding…" : "Add note"}
          </Button>
        </div>
      </form>
    </Card>
  );
}

export function EnquiryDetail() {
  const { id } = useParams<{ id: string }>();
  const user = useUser();
  const toast = useToast();
  const r = useLoad(() => api<Detail>("GET", `/v1/enquiries/${id}`), id);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [winning, setWinning] = useState(false);

  if (r.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (r.status === "error") return <Alert tone="danger" title={errorMessage(r.error)} />;
  const e = r.data;
  const isOpen = openStatuses.includes(e.status);

  const apply = async (to: Parameters<typeof change>[1], done?: string) => {
    setBusy(true);
    setError(null);
    try {
      r.set(await api<Detail>("PATCH", `/v1/enquiries/${e.id}`, change(e, to, user.id)));
      setWinning(false);
      if (done) toast({ tone: "success", title: done });
    } catch (x) {
      const ae = asError(x);
      if (ae.code === "enquiry_changed") {
        toast({ tone: "info", title: "Someone else changed this enquiry. It now shows their change." });
        r.reload();
      } else setError(errorMessage(ae));
    } finally {
      setBusy(false);
    }
  };

  const facts: Array<[string, React.ReactNode]> = [
    ["Name", e.name],
    [
      "Email",
      <a key="email" href={`mailto:${e.email}`} className="underline underline-offset-4">
        {e.email}
      </a>,
    ],
    ["Interested in", projectTypeLabel(e.project_type)],
    ["Languages", e.languages || "Not given"],
    ["Volume", volumeLabel(e.volume)],
    ["Timeline", timelineLabel(e.timeline)],
    ["Received", shortDate(e.created_at)],
  ];

  return (
    <div className="grid gap-6">
      <Breadcrumb items={[{ label: "Enquiries", href: "/w/enquiries" }, { label: e.organisation }]} />
      <PageTitle title={e.organisation}>
        <EnquiryStatusPill status={e.status} />
      </PageTitle>

      {e.status === "won" && (
        <Alert
          tone="success"
          title={e.org_name ? `Won, linked to ${e.org_name}` : "Won, not linked to an organisation yet"}
        />
      )}
      {error && <Alert tone="danger" title={error} />}

      <div className="grid gap-6 lg:grid-cols-3">
        <div className="grid content-start gap-6 lg:col-span-2">
          <Card className="p-5 text-sm leading-relaxed whitespace-pre-wrap">
            <h2 className="mb-2 font-display text-base font-semibold">Message</h2>
            {e.message || <span className="text-fg-muted">No message.</span>}
          </Card>
          <Notes
            enquiry={e}
            onAdded={(n) => r.set({ ...e, notes: [...e.notes, n], note_count: e.note_count + 1 })}
          />
        </div>
        <div className="order-first grid content-start gap-6 lg:order-none">
          <Card className="grid gap-4 p-5">
            <h2 className="font-display text-base font-semibold">Handling</h2>
            {isOpen ? (
              <Field label="Assigned to">
                <Select
                  value={e.assigned_to ?? ""}
                  disabled={busy}
                  onChange={(x) => apply({ assigned_to: x.target.value || null }, "Assignment saved")}
                >
                  <option value="">Nobody</option>
                  {e.assignees.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.id === user.id ? `${p.name} (you)` : p.name}
                    </option>
                  ))}
                </Select>
              </Field>
            ) : (
              <p className="text-sm">
                <span className="text-fg-muted">Handled by </span>
                {e.assigned_name ?? "nobody"}
              </p>
            )}
            {e.status === "won" && (
              <WonOrg
                enquiry={e}
                busy={busy}
                onChange={(org) => apply({ org_id: org }, "Organisation link saved")}
              />
            )}
            {buttonMoves(e.moves).length > 0 && (
              <div className="flex flex-wrap gap-2">
                {buttonMoves(e.moves).map((to) => (
                  <Button
                    key={to}
                    size="sm"
                    variant={to === "won" || to === "in_conversation" ? "primary" : "secondary"}
                    disabled={busy}
                    onClick={() =>
                      to === "won" ? setWinning(true) : apply({ status: to }, `${statusText[to].label}`)
                    }
                  >
                    {moveLabel(e.status, to)}
                  </Button>
                ))}
              </div>
            )}
          </Card>
          <Card className="p-5">
            <dl className="grid gap-3 text-sm">
              {facts.map(([k, v]) => (
                <div key={k} className="grid gap-0.5">
                  <dt className="text-fg-muted">{k}</dt>
                  <dd className="break-words">{v}</dd>
                </div>
              ))}
            </dl>
          </Card>
        </div>
      </div>
      <WonDialog
        open={winning}
        busy={busy}
        current={e.org_id}
        onClose={() => setWinning(false)}
        onConfirm={(org) => apply({ status: "won", org_id: org }, "Marked won")}
      />
    </div>
  );
}

function WonOrg({
  enquiry,
  busy,
  onChange,
}: {
  enquiry: Detail;
  busy: boolean;
  onChange: (org: string | null) => void;
}) {
  const can = useCan();
  const orgs = useLoad(
    () =>
      can("org.manage")
        ? api<{ organisations: Organisation[] }>("GET", "/v1/organisations").then((r) =>
            r.organisations.filter((o) => o.status === "active" || o.id === enquiry.org_id),
          )
        : Promise.resolve(null),
    enquiry.id,
  );
  if (orgs.status !== "ready" || !orgs.data) return null;
  return (
    <fieldset disabled={busy}>
      <OrgPicker orgs={orgs.data} value={enquiry.org_id ?? ""} onChange={(v) => onChange(v || null)} />
    </fieldset>
  );
}
