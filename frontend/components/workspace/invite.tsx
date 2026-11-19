"use client";

import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Alert, Card } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select } from "@/components/ui/input";
import { Table, type Column } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { api, ApiError, type Invitation } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";

type Option = { value: string; label: string };

/**
 * Ops inviting people (`user.manage`, Stage 19f): client users into an
 * organisation, or staff with no organisation. The person gets an email
 * whose link opens /accept-invitation. Pending invitations of the same kind
 * are listed with a way to cancel them.
 */
export function InvitePanel({
  title,
  roles,
  organisations,
}: {
  title: string;
  roles: Option[];
  /** Given for client roles: the invitation is into one of these. */
  organisations?: Option[];
}) {
  const toast = useToast();
  const [tick, setTick] = useState(0);
  const invites = useLoad(
    () => api<{ invitations: Invitation[] }>("GET", "/v1/invitations").then((r) => r.invitations),
    String(tick),
  );
  const [email, setEmail] = useState("");
  const [role, setRole] = useState(roles[0].value);
  const [org, setOrg] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const roleLabel = (r: string) => roles.find((o) => o.value === r)?.label ?? r;
  const orgLabel = (id?: string) => organisations?.find((o) => o.value === id)?.label ?? "";

  async function invite(e: FormEvent) {
    e.preventDefault();
    if (organisations && !org)
      return setError(new ApiError(422, "invalid_input", "org_id", "Choose the organisation."));
    setBusy(true);
    setError(null);
    try {
      await api("POST", "/v1/invitations", {
        email: email.trim(),
        role,
        ...(organisations ? { org_id: org } : {}),
      });
      toast({ tone: "success", title: `Invitation sent to ${email.trim()}` });
      setEmail("");
      setTick((t) => t + 1);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  async function revoke(inv: Invitation) {
    try {
      await api("DELETE", `/v1/invitations/${inv.id}`);
      toast({ tone: "success", title: `Invitation for ${inv.email} cancelled` });
      setTick((t) => t + 1);
    } catch (err) {
      toast({
        tone: "danger",
        title: errorMessage(err instanceof ApiError ? err : new ApiError(0, "network")),
      });
    }
  }

  const pending =
    invites.status === "ready"
      ? invites.data.filter(
          (i) => roles.some((r) => r.value === i.role) && Boolean(i.org_id) === Boolean(organisations),
        )
      : [];
  const columns: Column<Invitation>[] = [
    { key: "email", header: "Email", cell: (i) => i.email },
    { key: "role", header: "Role", cell: (i) => roleLabel(i.role) },
    ...(organisations
      ? [{ key: "org", header: "Organisation", cell: (i: Invitation) => orgLabel(i.org_id) }]
      : []),
    { key: "expires", header: "Link expires", cell: (i) => shortDate(i.expires_at) },
    {
      key: "actions",
      header: <span className="sr-only">Actions</span>,
      cell: (i) => (
        <Button variant="ghost" size="sm" onClick={() => revoke(i)}>
          Cancel<span className="sr-only"> the invitation for {i.email}</span>
        </Button>
      ),
    },
  ];

  return (
    <section className="grid gap-3" aria-label={title}>
      <Card className="p-5">
        <form onSubmit={invite} noValidate className="grid gap-4">
          <h2 className="font-display font-semibold">{title}</h2>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-[1fr_14rem_14rem]">
            <Field
              label="Work email"
              required
              error={error?.field === "email" ? errorMessage(error) : undefined}
            >
              <Input
                type="email"
                autoComplete="off"
                maxLength={254}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </Field>
            {organisations && (
              <Field
                label="Organisation"
                required
                error={error?.field === "org_id" ? errorMessage(error) : undefined}
              >
                <Select value={org} onChange={(e) => setOrg(e.target.value)}>
                  <option value="">Choose…</option>
                  {organisations.map((o) => (
                    <option key={o.value} value={o.value}>
                      {o.label}
                    </option>
                  ))}
                </Select>
              </Field>
            )}
            <Field label="Role" required>
              <Select value={role} onChange={(e) => setRole(e.target.value)}>
                {roles.map((r) => (
                  <option key={r.value} value={r.value}>
                    {r.label}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          {error && !error.field && <Alert tone="danger" title={errorMessage(error)} />}
          <div className="flex flex-wrap items-center gap-4">
            <Button type="submit" variant="secondary" disabled={busy || !email.trim()}>
              {busy ? "Sending…" : "Send invitation"}
            </Button>
            <p className="text-sm text-fg-muted">
              They get an email with a link to set a password. The link works for 7 days.
            </p>
          </div>
        </form>
      </Card>
      {pending.length > 0 && (
        <Table
          caption={`Pending invitations: ${title.toLowerCase()}`}
          columns={columns}
          rows={pending}
          rowKey={(i) => i.id}
          empty="No pending invitations."
        />
      )}
    </section>
  );
}
