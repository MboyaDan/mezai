"use client";

import { useState } from "react";
import { Switch } from "@/components/ui/choice";
import { Alert, Skeleton } from "@/components/ui/display";
import { useToast } from "@/components/ui/toast";
import { api, ApiError, type Schemas } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";

type Prefs = Schemas["NotificationPreferences"];
type Optional = "project_available" | "feedback_received";

const optional: Array<{ key: Optional; label: string; hint: string }> = [
  {
    key: "project_available",
    label: "New projects",
    hint: "When a project you qualify for opens.",
  },
  {
    key: "feedback_received",
    label: "Feedback",
    hint: "When a reviewer leaves feedback on your work. At most one email a day.",
  },
];

const alwaysLabel: Record<Prefs["always"][number], string> = {
  account: "Account emails: confirming your address, resetting your password",
  qualification_result: "Your qualification test results",
  payout_sent: "Payouts sent to your M-Pesa number, with the receipt",
};

/** Email choices on the profile page (Stage 16). Each switch saves at once. */
export function EmailChoices() {
  const toast = useToast();
  const prefs = useLoad(() => api<Prefs>("GET", "/v1/me/notifications"));
  const [saving, setSaving] = useState<Optional | null>(null);
  const [error, setError] = useState<string | null>(null);

  const change = async (current: Prefs, key: Optional, on: boolean) => {
    if (prefs.status !== "ready") return;
    setSaving(key);
    setError(null);
    prefs.set({ ...current, [key]: on }); // show the change at once; put it back if saving fails
    try {
      const next = await api<Prefs>("PUT", "/v1/me/notifications", {
        project_available: key === "project_available" ? on : current.project_available,
        feedback_received: key === "feedback_received" ? on : current.feedback_received,
      });
      prefs.set(next);
      toast({ tone: "success", title: on ? "You will get these emails" : "You will not get these emails" });
    } catch (e) {
      prefs.set(current);
      setError(errorMessage(e instanceof ApiError ? e : new ApiError(0, "network")));
    } finally {
      setSaving(null);
    }
  };

  return (
    <section
      id="notifications"
      aria-labelledby="notifications-h"
      className="grid scroll-mt-24 gap-5 border-t border-border pt-8 md:grid-cols-12 md:gap-10"
    >
      <div className="md:col-span-4">
        <h2 id="notifications-h" className="font-display text-lg font-semibold">
          Email
        </h2>
        <p className="mt-1 text-sm text-fg-muted">
          Choose which emails you get. Changes save straight away. We never send marketing email.
        </p>
      </div>
      <div className="grid gap-5 md:col-span-8">
        {prefs.status === "loading" && <Skeleton className="h-32 w-full" />}
        {prefs.status === "error" && <Alert tone="danger" title={errorMessage(prefs.error)} />}
        {prefs.status === "ready" && (
          <>
            <ul className="grid gap-4">
              {optional.map((o) => (
                <li key={o.key} className="grid gap-1">
                  <Switch
                    checked={prefs.data[o.key]}
                    disabled={saving !== null}
                    onCheckedChange={(on) => void change(prefs.data, o.key, on)}
                    label={o.label}
                    aria-describedby={`${o.key}-hint`}
                  />
                  <p id={`${o.key}-hint`} className="pl-14 text-sm text-fg-muted">
                    {o.hint}
                  </p>
                </li>
              ))}
            </ul>
            {error && <Alert tone="danger" title={error} />}
            <div className="grid gap-2">
              <h3 className="text-sm font-semibold">Always sent</h3>
              <ul className="grid list-disc gap-1 pl-5 text-sm text-fg-muted">
                {prefs.data.always.map((k) => (
                  <li key={k}>{alwaysLabel[k]}</li>
                ))}
              </ul>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
