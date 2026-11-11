"use client";

import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Alert, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select } from "@/components/ui/input";
import { Table, type Column } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { api, ApiError, type Invitation, type Member } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { roleLabel } from "@/lib/portal";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { PageTitle } from "./workspace";

const memberColumns: Column<Member>[] = [
  {
    key: "name",
    header: "Name",
    sortValue: (m) => m.name,
    cell: (m) => (
      <span className="grid">
        <span className="font-medium">{m.name}</span>
        <span className="text-xs text-fg-muted">{m.email}</span>
      </span>
    ),
  },
  { key: "role", header: "Role", cell: (m) => roleLabel[m.role] ?? m.role },
  {
    key: "status",
    header: "Status",
    cell: (m) =>
      m.status === "active" ? (
        <StatusPill status="success">Active</StatusPill>
      ) : (
        <StatusPill status="neutral">Disabled</StatusPill>
      ),
  },
  { key: "joined", header: "Joined", cell: (m) => shortDate(m.joined_at), sortValue: (m) => m.joined_at },
  { key: "last", header: "Last signed in", cell: (m) => shortDate(m.last_login_at) },
];

/** /w/team: a Client Admin's colleagues and invitations (org.user.invite). */
export function TeamPage() {
  const user = useUser();
  const toast = useToast();
  const members = useLoad(() => api<{ members: Member[] }>("GET", "/v1/team").then((r) => r.members));
  const [tick, setTick] = useState(0);
  const invites = useLoad(
    () => api<{ invitations: Invitation[] }>("GET", "/v1/invitations").then((r) => r.invitations),
    String(tick),
  );
  const [email, setEmail] = useState("");
  const [role, setRole] = useState("client_viewer");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  const invite = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api("POST", "/v1/invitations", { email: email.trim(), role, org_id: user.org_id });
      toast({ tone: "success", title: `Invitation sent to ${email.trim()}` });
      setEmail("");
      setTick((t) => t + 1);
    } catch (err) {
      setError(err instanceof ApiError ? err : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (inv: Invitation) => {
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
  };

  const inviteColumns: Column<Invitation>[] = [
    { key: "email", header: "Email", cell: (i) => i.email },
    { key: "role", header: "Role", cell: (i) => roleLabel[i.role] ?? i.role },
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
    <div className="grid gap-10">
      <PageTitle title="Team">
        People in your organisation who can sign in to MezAI. Client Admins can request projects, invite
        colleagues and see invoices; Client Viewers can follow projects and download deliverables.
      </PageTitle>

      <section aria-labelledby="members-h" className="grid gap-3">
        <h2 id="members-h" className="font-display text-lg font-semibold">
          Members
        </h2>
        {members.status === "loading" && <Skeleton className="h-32 w-full" />}
        {members.status === "error" && <Alert tone="danger" title={errorMessage(members.error)} />}
        {members.status === "ready" && (
          <Table
            caption="Members"
            hideCaption
            columns={memberColumns}
            rows={members.data}
            rowKey={(m) => m.id}
            empty="No members yet."
          />
        )}
        <p className="text-sm text-fg-muted">To remove someone or change their role, ask the MezAI team.</p>
      </section>

      <section aria-labelledby="invite-h" className="grid gap-3">
        <h2 id="invite-h" className="font-display text-lg font-semibold">
          Invite a colleague
        </h2>
        <Card className="p-5">
          <form onSubmit={invite} className="grid gap-4 md:grid-cols-[1fr_14rem_auto] md:items-end">
            <Field label="Work email" required>
              <Input
                type="email"
                autoComplete="off"
                required
                maxLength={254}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </Field>
            <Field label="Role" required>
              <Select value={role} onChange={(e) => setRole(e.target.value)}>
                <option value="client_viewer">Client Viewer</option>
                <option value="client_admin">Client Admin</option>
              </Select>
            </Field>
            <Button type="submit" disabled={busy || !email.trim()}>
              {busy ? "Sending…" : "Send invitation"}
            </Button>
          </form>
          {error && <Alert tone="danger" title={errorMessage(error)} className="mt-4" />}
          <p className="mt-4 text-sm text-fg-muted">
            They get an email with a link to set a password. The link works for 7 days.
          </p>
        </Card>
        {invites.status === "ready" && invites.data.length > 0 && (
          <Table
            caption="Pending invitations"
            columns={inviteColumns}
            rows={invites.data}
            rowKey={(i) => i.id}
            empty="No pending invitations."
          />
        )}
      </section>
    </div>
  );
}
