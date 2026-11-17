"use client";

import { useState } from "react";
import { Button, buttonClasses } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type Schemas } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { erasureWaitingOn } from "@/lib/privacy";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";

type Erasure = Schemas["Erasure"];

/**
 * Ops view of a contributor's data rights (Stage 17b, `privacy.manage`):
 * download their data for a request made by email or phone, and erase the
 * account once nothing is owed either way (A-088).
 */
export function PersonalDataPanel({
  contributorId,
  onErased,
}: {
  contributorId: string;
  onErased: () => void;
}) {
  const toast = useToast();
  const state = useLoad(
    () => api<Erasure>("GET", `/v1/contributors/${contributorId}/erasure`),
    contributorId,
  );
  const [open, setOpen] = useState(false);
  const [confirm, setConfirm] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  if (state.status === "loading") return <Skeleton className="h-40 w-full" />;
  if (state.status === "error") return <Alert tone="danger" title={errorMessage(state.error)} />;
  const e = state.data;
  const waiting = erasureWaitingOn(e.blockers);

  async function call(method: "POST" | "DELETE") {
    setBusy(true);
    setError(null);
    try {
      const next = await api<Erasure>(
        method,
        `/v1/contributors/${contributorId}/erasure`,
        method === "POST" ? { confirm: confirm.trim(), reason: reason.trim() } : undefined,
      );
      if (state.status === "ready") state.set(next);
      setOpen(false);
      toast({
        tone: "success",
        title: method === "POST" ? `${e.public_code}: personal data erased` : "Request closed",
      });
      if (method === "POST") onErased();
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  function erase() {
    if (confirm.trim() !== e.public_code)
      return setError(new ApiError(422, "invalid_input", "confirm", `Type ${e.public_code} to confirm.`));
    if (!reason.trim())
      return setError(new ApiError(422, "invalid_input", "reason", "Say how the request reached us."));
    void call("POST");
  }

  return (
    <Card className="p-5" role="region" aria-labelledby="personal-data-h">
      <div className="flex items-center justify-between gap-3">
        <h2 id="personal-data-h" className="font-display font-semibold">
          Personal data
        </h2>
        {e.erased_at ? (
          <StatusPill status="neutral">Erased</StatusPill>
        ) : e.request ? (
          <StatusPill status="warning">Deletion requested</StatusPill>
        ) : null}
      </div>

      {e.erased_at ? (
        <p className="mt-3 text-sm text-fg-muted">
          Erased on {shortDate(e.erased_at)}. Earnings, payouts and the audit trail remain under{" "}
          {e.public_code} only.
        </p>
      ) : (
        <>
          {e.request && (
            <p className="mt-3 text-sm">
              {e.request.by_contributor ? "They asked" : "Recorded"} on {shortDate(e.request.created_at)}.
              {e.request.note && (
                <span className="mt-1 block text-fg-muted">&ldquo;{e.request.note}&rdquo;</span>
              )}
            </p>
          )}
          {waiting.length > 0 ? (
            <div className="mt-3 text-sm">
              <p className="font-medium">Settle before erasing</p>
              <ul className="mt-1 list-disc pl-5 text-fg-muted">
                {waiting.map((w) => (
                  <li key={w}>{w}</li>
                ))}
              </ul>
              <p className="mt-1 text-fg-muted">
                Pause them so no new work starts, then pay or decide what is open.
              </p>
            </div>
          ) : (
            <p className="mt-3 text-sm text-fg-muted">Nothing is owed either way.</p>
          )}
          <div className="mt-4 flex flex-wrap gap-2">
            <a
              href={`/api/v1/contributors/${contributorId}/data-export`}
              download
              className={buttonClasses({ variant: "secondary", size: "sm" })}
            >
              Download their data
            </a>
            <Button size="sm" variant="danger" disabled={!e.can_erase} onClick={() => setOpen(true)}>
              Erase personal data
            </Button>
            {e.request && (
              <Button size="sm" variant="ghost" loading={busy && !open} onClick={() => void call("DELETE")}>
                Close request
              </Button>
            )}
          </div>
        </>
      )}
      {error && !open && <Alert tone="danger" title={errorMessage(error)} className="mt-3" />}

      <Dialog
        open={open}
        onClose={() => {
          setOpen(false);
          setError(null);
        }}
        title={`Erase ${e.public_code}?`}
        description="Their name, email, phone and ID numbers, profile, languages and CV are deleted and they can no longer sign in. Earnings, payouts and the audit trail stay under the contributor code. This cannot be undone."
        footer={
          <>
            <Button variant="secondary" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button variant="danger" loading={busy} onClick={erase}>
              Erase
            </Button>
          </>
        }
      >
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} className="mb-4" />}
        <div className="grid gap-4">
          <Field
            label={`Type ${e.public_code} to confirm`}
            error={error?.field === "confirm" ? errorMessage(error) : undefined}
            required
          >
            <Input value={confirm} onChange={(ev) => setConfirm(ev.target.value)} autoComplete="off" />
          </Field>
          <Field
            label="How the request reached us"
            hint="For example: in the app, or by email on 17 Nov. Kept in the audit log."
            error={error?.field === "reason" ? errorMessage(error) : undefined}
            required
          >
            <Textarea value={reason} onChange={(ev) => setReason(ev.target.value)} rows={2} maxLength={500} />
          </Field>
        </div>
      </Dialog>
    </Card>
  );
}
