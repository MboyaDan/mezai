"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Card, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type ContributorProfile, type Schemas } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { shortDate } from "./contributors";
import { useCan } from "./workspace";

export type IdentityChecks = Schemas["IdentityChecks"];

export const identityLabel: Record<IdentityChecks["status"], string> = {
  pending: "Not checked",
  verified: "Verified",
  rejected: "Rejected",
};

export const identityPill = (s: IdentityChecks["status"]) =>
  s === "verified" ? "success" : s === "rejected" ? "danger" : "warning";

const flagText = (f: IdentityChecks["flags"][number]) => {
  const what = f.kind === "mpesa_in_use" ? "M-Pesa number" : "National ID number";
  return f.raised_by_this ? `Tried to use the ${what} of` : `Their ${what} was tried by`;
};

/** Possible duplicates for the verification note rule (mirrors DecideIdentity). */
export const hasPossibleDuplicates = (c: IdentityChecks) => c.flags.length > 0 || c.same_name.length > 0;

/**
 * Ops view of one contributor's identity (D-078): the last 3 ID digits,
 * possible duplicates by contributor code, verify or reject with a note,
 * and the M-Pesa registered-name check done at the first payout.
 */
export function IdentityPanel({
  profile: p,
  checks: c,
  onChange,
}: {
  profile: ContributorProfile;
  checks: IdentityChecks;
  onChange: () => void;
}) {
  const can = useCan();
  const toast = useToast();
  const [decision, setDecision] = useState<"verify" | "reject" | null>(null);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const complete = Boolean(p.national_id_last3 && p.mpesa_phone_confirmed);
  const needsNote = decision === "reject" || (decision === "verify" && hasPossibleDuplicates(c));

  async function post(path: string, body: unknown, done: string) {
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/contributors/${p.id}/${path}`, body);
      toast({ tone: "success", title: done });
      setDecision(null);
      setNote("");
      onChange();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  function decide() {
    if (needsNote && !note.trim())
      return setError(
        new ApiError(
          422,
          "invalid_input",
          "note",
          decision === "reject"
            ? "Tell the contributor why. They see this note."
            : "Say why the possible duplicates are not the same person.",
        ),
      );
    void post(
      "identity",
      { decision, note: note.trim() },
      `${p.public_code}: identity ${decision === "verify" ? "verified" : "rejected"}`,
    );
  }

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between gap-3">
        <h2 className="font-display font-semibold">Identity</h2>
        <StatusPill status={identityPill(c.status)}>{identityLabel[c.status]}</StatusPill>
      </div>
      <dl className="mt-3 grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-sm">
        <dt className="text-fg-muted">National ID</dt>
        <dd className="font-mono">{c.national_id_last3 ? `••••• ${c.national_id_last3}` : "–"}</dd>
        <dt className="text-fg-muted">M-Pesa name</dt>
        <dd>
          {c.mpesa_name_checked_at
            ? `Matches · ${c.mpesa_name_checked_by ?? ""} · ${shortDate(c.mpesa_name_checked_at)}`
            : "Not checked yet (before the first payout)"}
        </dd>
        {c.decided_at && (
          <>
            <dt className="text-fg-muted">Decided</dt>
            <dd>
              {c.decided_by} · {shortDate(c.decided_at)}
            </dd>
          </>
        )}
      </dl>
      {c.note && <p className="mt-3 text-sm text-fg-muted">Note: {c.note}</p>}

      <div className="mt-4 grid gap-2 text-sm">
        <p className="font-medium">Possible duplicates</p>
        {!hasPossibleDuplicates(c) && <p className="text-fg-muted">None found.</p>}
        <ul className="grid gap-1.5">
          {c.flags.map((f, i) => (
            <li key={`f${i}`}>
              {flagText(f)}{" "}
              <Link href={`/w/contributors/${f.other.id}`} className="font-mono underline underline-offset-4">
                {f.other.public_code}
              </Link>{" "}
              <span className="text-fg-muted">· {shortDate(f.at)}</span>
            </li>
          ))}
          {c.same_name.map((o) => (
            <li key={o.id}>
              Same name as{" "}
              <Link href={`/w/contributors/${o.id}`} className="font-mono underline underline-offset-4">
                {o.public_code}
              </Link>{" "}
              <span className="text-fg-muted">({o.status})</span>
            </li>
          ))}
        </ul>
      </div>

      {(can("contributor.approve") || can("payout.create")) && (
        <div className="mt-5 flex flex-wrap gap-2">
          {can("contributor.approve") && c.status !== "verified" && (
            <Button size="sm" onClick={() => setDecision("verify")} disabled={!complete}>
              Verify identity
            </Button>
          )}
          {can("contributor.approve") && c.status !== "rejected" && (
            <Button size="sm" variant="secondary" onClick={() => setDecision("reject")}>
              Reject identity
            </Button>
          )}
          {can("payout.create") && p.mpesa_phone_confirmed && !c.mpesa_name_checked_at && (
            <Button
              size="sm"
              variant="secondary"
              loading={busy && decision === null}
              onClick={() => post("mpesa-name-check", {}, `${p.public_code}: M-Pesa name check recorded`)}
            >
              M-Pesa name matches
            </Button>
          )}
        </div>
      )}
      {!complete && (
        <p className="mt-2 text-sm text-fg-muted">
          Verification opens once they give a confirmed M-Pesa number and a National ID number.
        </p>
      )}
      {error && decision === null && <Alert tone="danger" title={errorMessage(error)} className="mt-3" />}

      <Dialog
        open={decision !== null}
        onClose={() => {
          setDecision(null);
          setError(null);
        }}
        title={decision === "verify" ? `Verify ${p.public_code}?` : `Reject ${p.public_code}'s identity?`}
        description={
          decision === "verify"
            ? "Confirm that no other account belongs to this person. If they have passed a test, they become active."
            : "They see your note and can correct their numbers. Their status does not change; pause them separately if needed."
        }
        footer={
          <>
            <Button variant="secondary" onClick={() => setDecision(null)}>
              Cancel
            </Button>
            <Button variant={decision === "reject" ? "danger" : "primary"} onClick={decide} loading={busy}>
              {decision === "verify" ? "Verify" : "Reject"}
            </Button>
          </>
        }
      >
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} className="mb-4" />}
        <Field
          label="Note"
          hint={
            decision === "reject"
              ? "Shown to the contributor. Kept in the audit log."
              : needsNote
                ? "Required: there are possible duplicates. Kept in the audit log."
                : "Optional. Kept in the audit log."
          }
          error={error?.field === "note" ? errorMessage(error) : undefined}
          required={needsNote}
        >
          <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} maxLength={1000} />
        </Field>
      </Dialog>
    </Card>
  );
}
