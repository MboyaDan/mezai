"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { StatusBadge } from "@/components/contributor/status";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Breadcrumb, Card, Skeleton, StatusPill, Tag } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { labelFor } from "@/content/profile";
import {
  ApiError,
  api,
  type AttemptSummary,
  type ContributorProfile,
  type ContributorStatus,
} from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { pct, shortDate } from "./contributors";
import { PageTitle, useCan } from "./workspace";

type Detail = {
  profile: ContributorProfile;
  history: Array<{ from: string | null; to: string; reason: string; changed_by: string | null; at: string }>;
  attempts: AttemptSummary[];
};

/** Manual changes Ops may make (mirrors contributors.CanTransition via Ops). */
export function manualTargets(from: ContributorStatus): Array<{ to: ContributorStatus; label: string }> {
  const out: Array<{ to: ContributorStatus; label: string }> = [];
  if (from === "active") out.push({ to: "paused", label: "Pause" });
  if (from === "paused") out.push({ to: "active", label: "Reinstate" });
  if (from !== "removed") out.push({ to: "removed", label: "Remove" });
  return out;
}

export function ContributorDetail() {
  const { id } = useParams<{ id: string }>();
  const can = useCan();
  const d = useLoad(() => api<Detail>("GET", `/v1/contributors/${id}`), id);
  const [target, setTarget] = useState<{ to: ContributorStatus; label: string } | null>(null);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const toast = useToast();

  if (d.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (d.status === "error") return <Alert tone="danger" title={errorMessage(d.error)} />;
  const { profile: p, history, attempts } = d.data;

  async function change() {
    if (!target) return;
    if (!reason.trim())
      return setError(
        new ApiError(422, "invalid_input", "reason", "Give a reason. It is kept in the audit log."),
      );
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/contributors/${p.id}/status`, { status: target.to, reason: reason.trim() });
      toast({ tone: "success", title: `${p.public_code}: ${target.label.toLowerCase()}d` });
      setTarget(null);
      setReason("");
      d.reload();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  const facts: Array<[string, React.ReactNode]> = [
    ["Email", p.email],
    ["Country", p.country ? labelFor(p.country) : "–"],
    ["City", p.city ?? "–"],
    ["Education", p.education_level ? labelFor(p.education_level) : "–"],
    ["Hours a week", p.weekly_hours ?? "–"],
    [
      "M-Pesa",
      <span key="m" className="font-mono">
        {p.mpesa_phone_masked || "–"}
      </span>,
    ],
    ["Joined", shortDate(p.created_at)],
  ];

  return (
    <div className="grid gap-6">
      <Breadcrumb
        items={[{ label: "Contributors", href: "/w/contributors?tab=all" }, { label: p.public_code }]}
      />
      <div className="flex flex-wrap items-start justify-between gap-4">
        <PageTitle
          title={
            <>
              <span className="font-mono">{p.public_code}</span> {p.display_name}
            </>
          }
        >
          <StatusBadge status={p.status} />
        </PageTitle>
        {can("contributor.pause") && (
          <div className="flex flex-wrap gap-2">
            {manualTargets(p.status).map((t) => (
              <Button
                key={t.to}
                variant={t.to === "removed" ? "danger" : "secondary"}
                size="sm"
                onClick={() => setTarget(t)}
              >
                {t.label}
              </Button>
            ))}
          </div>
        )}
      </div>

      <div className="grid gap-6 lg:grid-cols-3">
        <Card className="p-5 lg:col-span-1">
          <h2 className="font-display font-semibold">Profile</h2>
          <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
            {facts.map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-fg-muted">{k}</dt>
                <dd className="min-w-0 truncate">{v}</dd>
              </div>
            ))}
          </dl>
          <div className="mt-4 grid gap-2 text-sm">
            <p className="text-fg-muted">Languages</p>
            <ul className="flex flex-wrap gap-1.5">
              {p.languages.map((l) => (
                <li key={l.language}>
                  <Tag>
                    {labelFor(l.language)} · {l.proficiency}
                    {l.dialects.length > 0 && ` (${l.dialects.join(", ")})`}
                  </Tag>
                </li>
              ))}
              {p.languages.length === 0 && <li>–</li>}
            </ul>
            <p className="text-fg-muted">Domains</p>
            <p>{p.domains.map(labelFor).join(", ") || "–"}</p>
            {p.missing.length > 0 && (
              <p className="text-fg-muted">Profile incomplete: {p.missing.length} fields missing.</p>
            )}
          </div>
        </Card>

        <div className="grid content-start gap-6 lg:col-span-2">
          <section aria-labelledby="attempts-h">
            <h2 id="attempts-h" className="mb-3 font-display font-semibold">
              Qualification attempts
            </h2>
            <Table
              caption="Qualification attempts"
              hideCaption
              rows={attempts}
              rowKey={(a) => a.id}
              empty="No attempts yet."
              columns={[
                { key: "test", header: "Test", cell: (a) => a.test_title },
                { key: "score", header: "Score", numeric: true, cell: (a) => pct(a.score) },
                {
                  key: "status",
                  header: "Status",
                  cell: (a) => (
                    <StatusPill
                      status={
                        a.status === "approved" ? "success" : a.status === "rejected" ? "danger" : "info"
                      }
                    >
                      {a.status === "scored"
                        ? "Waiting"
                        : a.status === "in_progress"
                          ? "In progress"
                          : a.status[0].toUpperCase() + a.status.slice(1)}
                    </StatusPill>
                  ),
                },
                {
                  key: "go",
                  header: <span className="sr-only">Open</span>,
                  cell: (a) =>
                    a.status !== "in_progress" && can("contributor.approve") ? (
                      <Link
                        href={`/w/contributors/attempts/${a.id}`}
                        className="underline underline-offset-4"
                      >
                        Open
                      </Link>
                    ) : null,
                },
              ]}
            />
          </section>
          <section aria-labelledby="history-h">
            <h2 id="history-h" className="mb-3 font-display font-semibold">
              Status history
            </h2>
            <ol className="grid gap-3 border-l border-border pl-5">
              {history.map((h, i) => (
                <li key={i} className="grid gap-0.5 text-sm">
                  <span>
                    {h.from ? `${h.from} → ` : ""}
                    <strong>{h.to}</strong>
                  </span>
                  <span className="text-fg-muted">
                    {h.reason} · {h.changed_by ?? "System"} · {shortDate(h.at)}
                  </span>
                </li>
              ))}
            </ol>
          </section>
        </div>
      </div>

      <Dialog
        open={target !== null}
        onClose={() => {
          setTarget(null);
          setError(null);
        }}
        title={target ? `${target.label} ${p.public_code}?` : ""}
        description={
          target?.to === "removed"
            ? "Removal is final. They are signed out everywhere; their history and ledger are kept."
            : target?.to === "paused"
              ? "They cannot take tests or tasks until reinstated."
              : "They can take tests and tasks again."
        }
        footer={
          <>
            <Button variant="secondary" onClick={() => setTarget(null)}>
              Cancel
            </Button>
            <Button variant={target?.to === "removed" ? "danger" : "primary"} onClick={change} loading={busy}>
              {target?.label}
            </Button>
          </>
        }
      >
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} className="mb-4" />}
        <Field
          label="Reason"
          hint="Kept in the audit log."
          error={error?.field === "reason" ? errorMessage(error) : undefined}
          required
        >
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} maxLength={1000} />
        </Field>
      </Dialog>
    </div>
  );
}
