"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/choice";
import { Alert, Card } from "@/components/ui/display";
import { ApiError, api, type ContributorProfile } from "@/lib/api/client";
import { consentPayload, errorMessage, legalFor } from "@/lib/onboarding";
import { useProfile } from "./portal";

/** Shown instead of the portal when a required document has a new version. */
export function ConsentPrompt() {
  const { profile, setProfile } = useProfile();
  const keys = profile.consent_required;
  const [agreed, setAgreed] = useState<Record<string, boolean>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const all = keys.every((k) => agreed[k]);

  async function accept() {
    setBusy(true);
    setError(null);
    try {
      setProfile(
        await api<ContributorProfile>("POST", "/v1/me/contributor/consents", {
          consents: consentPayload(keys),
        }),
      );
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section aria-labelledby="consent-title" className="mx-auto grid max-w-2xl gap-6">
      <div className="grid gap-2">
        <h1
          id="consent-title"
          className="font-display text-[1.75rem] leading-tight font-medium tracking-[-0.02em]"
        >
          Please review the updated documents
        </h1>
        <p className="text-fg-muted">
          We changed the documents below. Read them and accept the new versions to keep using the portal. We
          record which version you accepted and when.
        </p>
      </div>
      {error && <Alert tone="danger" title={errorMessage(error)} />}
      <Card className="grid gap-4 p-5">
        {keys.map((k) => {
          const doc = legalFor(k);
          return (
            <Checkbox
              key={k}
              checked={!!agreed[k]}
              onChange={(e) => setAgreed((a) => ({ ...a, [k]: e.target.checked }))}
              label={
                <>
                  I have read and accept the{" "}
                  <Link href={`/legal/${doc.slug}`} target="_blank" className="underline underline-offset-2">
                    {doc.short.toLowerCase()}
                  </Link>{" "}
                  <span className="font-mono text-xs text-fg-muted">v{doc.version}</span>
                </>
              }
            />
          );
        })}
      </Card>
      <div>
        <Button onClick={accept} disabled={!all} loading={busy}>
          Accept and continue
        </Button>
      </div>
    </section>
  );
}
