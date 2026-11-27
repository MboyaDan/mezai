"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Card, Skeleton } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type Schemas } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { retentionSentence } from "@/lib/privacy";
import { usePreloaded } from "@/lib/preload";
import { useLoad } from "@/lib/use-load";
import { useCan } from "../workspace";
import { useProject } from "./frame";

type Retention = Schemas["Retention"];

/**
 * When the project's task content is deleted (brief 12), for Ops and the
 * client. Ops with `privacy.manage` can delete a closed project's content at
 * once, for example when the client asks.
 */
export function RetentionCard() {
  const { project: p } = useProject();
  const can = useCan();
  const toast = useToast();
  const r = useLoad(
    () => api<Retention>("GET", `/v1/projects/${p.id}/retention`),
    `${p.id}:${p.status}`,
    usePreloaded<Retention>(`/v1/projects/${p.id}/retention`),
  );
  const [open, setOpen] = useState(false);
  const [confirm, setConfirm] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  if (r.status === "loading") return <Skeleton className="h-28 w-full" />;
  if (r.status === "error") return <Alert tone="danger" title={errorMessage(r.error)} />;

  async function purge() {
    if (confirm.trim() !== p.name)
      return setError(
        new ApiError(422, "invalid_input", "confirm", "Type the project name exactly as shown."),
      );
    if (!reason.trim())
      return setError(new ApiError(422, "invalid_input", "reason", "Say who asked and how."));
    setBusy(true);
    setError(null);
    try {
      const out = await api<{ retention: Retention }>("POST", `/v1/projects/${p.id}/purge`, {
        confirm: confirm.trim(),
        reason: reason.trim(),
      });
      if (r.status === "ready") r.set(out.retention);
      setOpen(false);
      toast({
        tone: "success",
        title: "Task content deleted",
        body: "Counts, quality figures and earnings are kept.",
      });
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  const canPurge = can("privacy.manage") && p.status === "closed" && !r.data.purged_at;
  return (
    <Card className="p-5" role="region" aria-labelledby="retention-h">
      <h2 id="retention-h" className="font-display font-semibold">
        Data retention
      </h2>
      <p className="mt-2 text-sm">{retentionSentence(r.data)}</p>
      <p className="mt-1 text-sm text-fg-muted">
        Counts, quality figures and the record of who did what are kept; item text, answers and files are not.
      </p>
      {canPurge && (
        <Button size="sm" variant="danger" className="mt-4" onClick={() => setOpen(true)}>
          Delete task content now
        </Button>
      )}
      <Dialog
        open={open}
        onClose={() => {
          setOpen(false);
          setError(null);
        }}
        title="Delete this project's task content now?"
        description="Item text, every answer, gold answers, review notes and the deliverable and import files are deleted. Qualification tests made from this project's gold are retired. This cannot be undone."
        footer={
          <>
            <Button variant="secondary" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button variant="danger" loading={busy} onClick={() => void purge()}>
              Delete content
            </Button>
          </>
        }
      >
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} className="mb-4" />}
        <div className="grid gap-4">
          <Field
            label={`Type the project name: ${p.name}`}
            error={error?.field === "confirm" ? errorMessage(error) : undefined}
            required
          >
            <Input value={confirm} onChange={(e) => setConfirm(e.target.value)} autoComplete="off" />
          </Field>
          <Field
            label="Who asked, and how"
            hint="For example: the client's admin, by email on 17 Nov. Kept in the audit log."
            error={error?.field === "reason" ? errorMessage(error) : undefined}
            required
          >
            <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={2} maxLength={500} />
          </Field>
        </div>
      </Dialog>
    </Card>
  );
}
