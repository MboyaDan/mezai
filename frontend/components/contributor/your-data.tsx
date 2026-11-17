"use client";

import { useState } from "react";
import { Button, buttonClasses } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Skeleton } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { api, ApiError, type Schemas } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { erasureWaitingOn } from "@/lib/privacy";
import { useLoad } from "@/lib/use-load";

type Erasure = Schemas["Erasure"];

const day = (iso: string) =>
  new Date(iso).toLocaleDateString("en-KE", { day: "numeric", month: "long", year: "numeric" });

/**
 * "Your data" on the profile page (Stage 17b): a copy of everything held
 * about the contributor, and a request to delete the account (DPA 2019).
 */
export function YourData() {
  const toast = useToast();
  const state = useLoad(() => api<Erasure>("GET", "/v1/me/erasure"));
  const [asking, setAsking] = useState(false);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function send(method: "POST" | "DELETE") {
    setBusy(true);
    setError(null);
    try {
      const next = await api<Erasure>(
        method,
        "/v1/me/erasure",
        method === "POST" ? { note: note.trim() } : undefined,
      );
      if (state.status === "ready") state.set(next);
      setAsking(false);
      setNote("");
      toast({
        tone: "success",
        title: method === "POST" ? "Request sent to the MezAI team" : "Request withdrawn",
      });
    } catch (e) {
      setError(errorMessage(e instanceof ApiError ? e : new ApiError(0, "network")));
    } finally {
      setBusy(false);
    }
  }

  const waiting = state.status === "ready" ? erasureWaitingOn(state.data.blockers) : [];

  return (
    <section
      id="your-data"
      aria-labelledby="your-data-h"
      className="grid scroll-mt-24 gap-5 border-t border-border pt-8 md:grid-cols-12 md:gap-10"
    >
      <div className="md:col-span-4">
        <h2 id="your-data-h" className="font-display text-lg font-semibold">
          Your data
        </h2>
        <p className="mt-1 text-sm text-fg-muted">
          Under Kenya&rsquo;s Data Protection Act you can have a copy of what we hold about you, and ask us to
          delete it.
        </p>
      </div>
      <div className="grid gap-8 md:col-span-8">
        <div className="grid gap-2">
          <h3 className="text-sm font-semibold">Download a copy</h3>
          <p className="text-sm text-fg-muted">
            One file with your account, profile, test scores, work history, earnings and payouts. The task
            content you worked on belongs to clients, so it is not included.
          </p>
          <div>
            <a
              href="/api/v1/me/data-export"
              download
              className={buttonClasses({ variant: "secondary", size: "sm" })}
            >
              Download my data
            </a>
          </div>
        </div>

        <div className="grid gap-2">
          <h3 className="text-sm font-semibold">Delete my account</h3>
          {state.status === "loading" && <Skeleton className="h-16 w-full" />}
          {state.status === "error" && <Alert tone="danger" title={errorMessage(state.error)} />}
          {state.status === "ready" && !state.data.request && (
            <>
              <p className="text-sm text-fg-muted">
                We delete your name, contact details, numbers, profile and CV, and you can no longer sign in.
                Records of your earnings and payouts stay for 7 years, as tax law requires, under your
                contributor code only.
              </p>
              <div>
                <Button variant="secondary" size="sm" onClick={() => setAsking(true)}>
                  Ask us to delete my account
                </Button>
              </div>
            </>
          )}
          {state.status === "ready" && state.data.request && (
            <Alert
              tone="info"
              title={`You asked us to delete your account on ${day(state.data.request.created_at)}`}
            >
              {waiting.length === 0 ? (
                <p>Nothing is outstanding. The MezAI team will delete it soon, and you will be signed out.</p>
              ) : (
                <>
                  <p>We delete it once these are settled, so nothing you are owed is lost:</p>
                  <ul className="mt-1 list-disc pl-5">
                    {waiting.map((w) => (
                      <li key={w}>{w}</li>
                    ))}
                  </ul>
                </>
              )}
              <Button
                variant="secondary"
                size="sm"
                className="mt-3"
                loading={busy}
                onClick={() => void send("DELETE")}
              >
                Withdraw request
              </Button>
            </Alert>
          )}
          {error && !asking && <Alert tone="danger" title={error} />}
        </div>
      </div>

      <Dialog
        open={asking}
        onClose={() => {
          setAsking(false);
          setError(null);
        }}
        title="Ask us to delete your account?"
        description="The MezAI team deletes it once nothing is owed either way. You can withdraw the request until then."
        footer={
          <>
            <Button variant="secondary" onClick={() => setAsking(false)}>
              Cancel
            </Button>
            <Button variant="danger" loading={busy} onClick={() => void send("POST")}>
              Send request
            </Button>
          </>
        }
      >
        {error && <Alert tone="danger" title={error} className="mb-4" />}
        {waiting.length > 0 && (
          <Alert tone="warning" title="Still open on your account" className="mb-4">
            <ul className="list-disc pl-5">
              {waiting.map((w) => (
                <li key={w}>{w}</li>
              ))}
            </ul>
          </Alert>
        )}
        <Field label="Anything we should know" hint="Optional. Only the MezAI team sees it.">
          <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} maxLength={1000} />
        </Field>
      </Dialog>
    </section>
  );
}
