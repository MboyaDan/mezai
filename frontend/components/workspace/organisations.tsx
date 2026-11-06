"use client";

import Link from "next/link";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type Organisation } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { PageTitle, useCan } from "./workspace";

function Create({ onCreated }: { onCreated: () => void }) {
  const [name, setName] = useState("");
  const [slug, setSlug] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const toast = useToast();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim())
      return setError(new ApiError(422, "invalid_input", "name", "Enter the organisation's name."));
    setBusy(true);
    setError(null);
    try {
      const o = await api<Organisation>("POST", "/v1/organisations", {
        name: name.trim(),
        slug: slug.trim() || undefined,
      });
      toast({ tone: "success", title: `${o.name} added` });
      setName("");
      setSlug("");
      onCreated();
    } catch (x) {
      setError(x instanceof ApiError ? x : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  const fieldErr = (f: string) =>
    error?.field === f || (f === "slug" && error?.code === "slug_taken") ? errorMessage(error!) : undefined;
  return (
    <Card className="p-5">
      <form onSubmit={submit} noValidate className="grid gap-4">
        <h2 className="font-display font-semibold">Add an organisation</h2>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Name" required error={fieldErr("name")}>
            <Input value={name} maxLength={200} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field
            label="Short name"
            hint="Lowercase letters, numbers and hyphens. Made from the name if left empty."
            error={fieldErr("slug")}
          >
            <Input value={slug} maxLength={60} onChange={(e) => setSlug(e.target.value.toLowerCase())} />
          </Field>
        </div>
        {error && !error.field && error.code !== "slug_taken" && (
          <Alert tone="danger" title={errorMessage(error)} />
        )}
        <div>
          <Button type="submit" variant="secondary" disabled={busy}>
            {busy ? "Adding…" : "Add organisation"}
          </Button>
        </div>
      </form>
    </Card>
  );
}

export function Organisations() {
  const can = useCan();
  const list = useLoad(() =>
    api<{ organisations: Organisation[] }>("GET", "/v1/organisations").then((r) => r.organisations),
  );
  return (
    <>
      <PageTitle title="Organisations">
        Client organisations. Each one only ever sees its own projects.
      </PageTitle>
      <div className="grid gap-6">
        {list.status === "loading" && <Skeleton className="h-40 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" && (
          <Table
            caption="Client organisations"
            hideCaption
            rows={list.data}
            rowKey={(o) => o.id}
            empty="No client organisations yet."
            columns={[
              { key: "name", header: "Name", cell: (o) => o.name, sortValue: (o) => o.name.toLowerCase() },
              {
                key: "slug",
                header: "Short name",
                cell: (o) => <span className="font-mono text-xs">{o.slug}</span>,
              },
              {
                key: "status",
                header: "Status",
                cell: (o) => (
                  <StatusPill status={o.status === "active" ? "success" : "neutral"}>
                    {o.status === "active" ? "Active" : "Archived"}
                  </StatusPill>
                ),
              },
              {
                key: "projects",
                header: "Projects",
                numeric: true,
                cell: (o) => o.project_count,
                sortValue: (o) => o.project_count,
              },
              {
                key: "members",
                header: "Users",
                numeric: true,
                cell: (o) => o.member_count,
                sortValue: (o) => o.member_count,
              },
              {
                key: "at",
                header: "Added",
                cell: (o) => shortDate(o.created_at),
                sortValue: (o) => o.created_at,
              },
              {
                key: "go",
                header: <span className="sr-only">Action</span>,
                cell: (o) =>
                  can("project.manage") && o.status === "active" ? (
                    <Link href={`/w/projects/new?org=${o.id}`} className="underline underline-offset-4">
                      New project<span className="sr-only"> for {o.name}</span>
                    </Link>
                  ) : null,
              },
            ]}
          />
        )}
        {can("org.manage") && <Create onCreated={list.reload} />}
      </div>
    </>
  );
}
