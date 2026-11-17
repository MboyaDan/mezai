"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type Schemas } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { staffOnly, staffRoleLabel } from "@/lib/two-step";
import { useLoad } from "@/lib/use-load";
import { useUser } from "@/lib/session";
import { shortDate } from "./contributors";
import { PageTitle } from "./workspace";

type User = Schemas["UserSummary"];

/**
 * Staff accounts and their two-step sign-in (Stage 17d, `user.manage`).
 * Resetting is for a lost phone: confirm who is asking before you do it.
 */
export function StaffPage() {
  const me = useUser();
  const toast = useToast();
  const list = useLoad(() => api<{ users: User[] }>("GET", "/v1/users").then((r) => staffOnly(r.users)));
  const [target, setTarget] = useState<User | null>(null);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  async function reset() {
    if (!target) return;
    if (!reason.trim())
      return setError(new ApiError(422, "invalid_input", "reason", "Say how you confirmed who is asking."));
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/users/${target.id}/two-step/reset`, { reason: reason.trim() });
      toast({ tone: "success", title: `Two-step sign-in reset for ${target.display_name}` });
      setTarget(null);
      setReason("");
      list.reload();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <PageTitle title="Staff">ELS staff accounts and whether two-step sign-in is on.</PageTitle>
      {list.status === "loading" && <Skeleton className="h-48 w-full" />}
      {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
      {list.status === "ready" && (
        <Table
          caption="Staff accounts"
          hideCaption
          rows={list.data}
          rowKey={(u) => u.id}
          empty="No staff accounts."
          columns={[
            { key: "name", header: "Name", cell: (u) => u.display_name, sortValue: (u) => u.display_name },
            { key: "email", header: "Email", cell: (u) => u.email },
            { key: "roles", header: "Roles", cell: (u) => u.roles.map(staffRoleLabel).join(", ") || "–" },
            {
              key: "two",
              header: "Two-step",
              cell: (u) =>
                u.two_step ? (
                  <StatusPill status="success">On</StatusPill>
                ) : (
                  <StatusPill status="neutral">Off</StatusPill>
                ),
              sortValue: (u) => (u.two_step ? 1 : 0),
            },
            { key: "last", header: "Last signed in", cell: (u) => shortDate(u.last_login_at) },
            {
              key: "act",
              header: <span className="sr-only">Actions</span>,
              cell: (u) =>
                u.two_step && u.id !== me.id ? (
                  <Button size="sm" variant="secondary" onClick={() => setTarget(u)}>
                    Reset two-step
                  </Button>
                ) : null,
            },
          ]}
        />
      )}
      <Dialog
        open={target !== null}
        onClose={() => {
          setTarget(null);
          setError(null);
        }}
        title={`Reset two-step sign-in for ${target?.display_name ?? ""}?`}
        description="Use this when someone has lost their phone. Their password alone will sign them in until they turn two-step on again. Confirm it is really them first, for example by calling them."
        footer={
          <>
            <Button variant="secondary" onClick={() => setTarget(null)}>
              Cancel
            </Button>
            <Button variant="danger" loading={busy} onClick={() => void reset()}>
              Reset
            </Button>
          </>
        }
      >
        {error && !error.field && <Alert tone="danger" title={errorMessage(error)} className="mb-4" />}
        <Field
          label="How you confirmed who is asking"
          hint="Kept in the audit log."
          error={error?.field === "reason" ? errorMessage(error) : undefined}
          required
        >
          <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={2} maxLength={500} />
        </Field>
      </Dialog>
    </>
  );
}
