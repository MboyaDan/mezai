"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { ChoiceGroup, Radio } from "@/components/ui/choice";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Select, Textarea } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type ContentReport } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { reportLabel } from "@/lib/tasks";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { PageTitle } from "./workspace";

type Status = "open" | "actioned" | "dismissed";
const statusText: Record<Status, string> = { open: "Open", actioned: "Withdrawn", dismissed: "Dismissed" };

/** /w/reports: content contributors reported (D-097). Open reports hold the item from everyone. */
export function Reports() {
  const [status, setStatus] = useState<Status>("open");
  const list = useLoad(
    () =>
      api<{ reports: ContentReport[] }>("GET", `/v1/content-reports?status=${status}`).then((r) => r.reports),
    status,
  );
  const [target, setTarget] = useState<ContentReport | null>(null);
  return (
    <>
      <PageTitle title="Content reports">
        Items contributors reported. While a report is open, nobody is given the item. Look at the item on its
        project, then dismiss the report to return the item to the queue, or withdraw the item for good.
      </PageTitle>
      <div className="grid gap-4">
        <Field label="Status" className="max-w-56">
          <Select value={status} onChange={(e) => setStatus(e.target.value as Status)}>
            {(Object.keys(statusText) as Status[]).map((s) => (
              <option key={s} value={s}>
                {statusText[s]}
              </option>
            ))}
          </Select>
        </Field>
        {list.status === "loading" && <Skeleton className="h-40 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" && (
          <Table
            caption="Content reports, oldest first"
            hideCaption
            rows={list.data}
            rowKey={(r) => r.id}
            empty={status === "open" ? "No open reports." : "None."}
            columns={[
              {
                key: "at",
                header: "Reported",
                cell: (r) => shortDate(r.created_at),
                sortValue: (r) => r.created_at,
              },
              {
                key: "project",
                header: "Project",
                cell: (r) => (
                  <Link href={`/w/projects/${r.project_id}/items`} className="underline underline-offset-4">
                    {r.project_name}
                  </Link>
                ),
              },
              {
                key: "item",
                header: "Item",
                cell: (r) => <span className="font-mono text-xs">{r.external_id ?? "–"}</span>,
              },
              {
                key: "what",
                header: "Problem",
                cell: (r) => (
                  <span className="grid gap-0.5">
                    <span>{reportLabel[r.category] ?? r.category}</span>
                    {r.note && <span className="text-sm text-fg-muted">“{r.note}”</span>}
                  </span>
                ),
              },
              {
                key: "by",
                header: "By",
                cell: (r) => <span className="font-mono text-xs">{r.reported_by}</span>,
              },
              {
                key: "go",
                header: <span className="sr-only">Action</span>,
                cell: (r) =>
                  r.status === "open" ? (
                    <Button size="sm" variant="secondary" onClick={() => setTarget(r)}>
                      Resolve<span className="sr-only"> report on {r.external_id ?? "item"}</span>
                    </Button>
                  ) : (
                    <StatusPill status={r.status === "actioned" ? "danger" : "neutral"}>
                      {statusText[r.status]}
                    </StatusPill>
                  ),
              },
            ]}
          />
        )}
      </div>
      <ResolveDialog report={target} onClose={() => setTarget(null)} onDone={list.reload} />
    </>
  );
}

function ResolveDialog({
  report,
  onClose,
  onDone,
}: {
  report: ContentReport | null;
  onClose: () => void;
  onDone: () => void;
}) {
  const toast = useToast();
  const [action, setAction] = useState<"dismiss" | "withdraw" | "">("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState<ApiError | null>(null);
  const [busy, setBusy] = useState(false);
  function close() {
    setAction("");
    setReason("");
    setError(null);
    onClose();
  }
  async function resolve() {
    if (!report) return;
    if (!action) return setError(new ApiError(422, "invalid_input", "action", "Choose what to do."));
    if (!reason.trim())
      return setError(
        new ApiError(422, "invalid_input", "reason", "Give a reason. It is kept in the audit log."),
      );
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/content-reports/${report.id}/resolve`, { action, reason: reason.trim() });
      toast({ tone: "success", title: action === "dismiss" ? "Report dismissed" : "Item withdrawn" });
      close();
      onDone();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }
  return (
    <Dialog
      open={report !== null}
      onClose={close}
      title="Resolve report"
      description={
        report ? `${reportLabel[report.category] ?? report.category} · ${report.project_name}` : ""
      }
      footer={
        <>
          <Button variant="secondary" onClick={close}>
            Cancel
          </Button>
          <Button variant={action === "withdraw" ? "danger" : "primary"} onClick={resolve} loading={busy}>
            {action === "withdraw" ? "Withdraw item" : "Resolve"}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} />}
        <ChoiceGroup
          legend="What should happen to the item?"
          error={error?.field === "action" ? errorMessage(error) : undefined}
        >
          <Radio
            name="resolve-action"
            checked={action === "dismiss"}
            onChange={() => setAction("dismiss")}
            label="Dismiss the report"
            hint="The item is fine. It goes back into the queue."
          />
          <Radio
            name="resolve-action"
            checked={action === "withdraw"}
            onChange={() => setAction("withdraw")}
            label="Withdraw the item"
            hint="Nobody works on it again. If it was a gold item, it is retired."
          />
        </ChoiceGroup>
        <Field
          label="Reason"
          hint="Kept in the audit log."
          error={error?.field === "reason" ? errorMessage(error) : undefined}
          required
        >
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={2} maxLength={1000} />
        </Field>
      </div>
    </Dialog>
  );
}
