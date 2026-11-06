"use client";

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Button, ButtonLink } from "@/components/ui/button";
import { Checkbox, ChoiceGroup, Radio } from "@/components/ui/choice";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Breadcrumb, Card, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input, Select, Textarea } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { labelFor, languageOptions } from "@/content/profile";
import { ApiError, api, upload, type ProjectRequest, type ProjectRequestInput } from "@/lib/api/client";
import { errorMessage, type TaskType } from "@/lib/onboarding";
import { emptyForm, fileSize, taskTypeLabel } from "@/lib/projects";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "./contributors";
import { ProjectFormView } from "./projects/form";
import { PageTitle, useCan } from "./workspace";

type Status = ProjectRequest["status"];
const statusText: Record<Status, { label: string; tone: "info" | "success" | "neutral" }> = {
  submitted: { label: "Waiting for MezAI", tone: "info" },
  converted: { label: "Project created", tone: "success" },
  declined: { label: "Declined", tone: "neutral" },
};
const requestTypes = ["classification", "pairwise", "translation_review", "other"] as const;
const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

function RequestStatus({ status }: { status: Status }) {
  return <StatusPill status={statusText[status].tone}>{statusText[status].label}</StatusPill>;
}

export function Requests() {
  const user = useUser();
  const can = useCan();
  const staff = user.kind === "staff";
  const [status, setStatus] = useState<Status | "">(staff ? "submitted" : "");
  const list = useLoad(
    () =>
      api<{ requests: ProjectRequest[] }>(
        "GET",
        `/v1/project-requests${status ? `?status=${status}` : ""}`,
      ).then((r) => r.requests),
    status,
  );
  return (
    <>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <PageTitle title="Project requests">
          {staff
            ? "Client requests for new work. Convert one into a draft project, or decline it with a note."
            : "Tell MezAI about work you need. We reply and set up the project with you."}
        </PageTitle>
        {can("org.project.request") && !staff && (
          <ButtonLink href="/w/requests/new">Request a project</ButtonLink>
        )}
      </div>
      <div className="grid gap-4">
        <Field label="Status" className="max-w-56">
          <Select value={status} onChange={(e) => setStatus(e.target.value as Status | "")}>
            <option value="">All</option>
            {(Object.keys(statusText) as Status[]).map((s) => (
              <option key={s} value={s}>
                {statusText[s].label}
              </option>
            ))}
          </Select>
        </Field>
        {list.status === "loading" && <Skeleton className="h-40 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" && (
          <Table
            caption="Project requests, newest first"
            hideCaption
            rows={list.data}
            rowKey={(r) => r.id}
            empty={
              <EmptyState title={staff ? "No requests with this status" : "No requests yet"}>
                {staff ? "Client admins submit requests from their workspace." : undefined}
              </EmptyState>
            }
            columns={[
              {
                key: "title",
                header: "Request",
                cell: (r) => (
                  <Link href={`/w/requests/${r.id}`} className="font-medium underline underline-offset-4">
                    {r.title}
                  </Link>
                ),
                sortValue: (r) => r.title.toLowerCase(),
              },
              ...(staff
                ? [
                    {
                      key: "org",
                      header: "Organisation",
                      cell: (r: ProjectRequest) => r.org_name,
                      sortValue: (r: ProjectRequest) => r.org_name,
                    },
                  ]
                : []),
              { key: "type", header: "Task type", cell: (r) => taskTypeLabel[r.task_type] },
              { key: "status", header: "Status", cell: (r) => <RequestStatus status={r.status} /> },
              {
                key: "at",
                header: "Submitted",
                cell: (r) => shortDate(r.created_at),
                sortValue: (r) => r.created_at,
              },
            ]}
          />
        )}
      </div>
    </>
  );
}

const sampleTypes = ".csv,.tsv,.jsonl,.json,.txt";

export function NewRequest() {
  const user = useUser();
  const router = useRouter();
  const toast = useToast();
  const [f, setF] = useState({
    title: "",
    task_type: "" as ProjectRequestInput["task_type"] | "",
    brief_md: "",
    languages: [] as string[],
    volume: "",
    wanted_by: "",
    contact_name: user.display_name,
    contact_email: user.email,
  });
  const [sample, setSample] = useState<File | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [apiError, setApiError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const up = (p: Partial<typeof f>) => setF((x) => ({ ...x, ...p }));

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const errs: Record<string, string> = {};
    if (!f.title.trim()) errs.title = "Give the request a short title.";
    if (!f.task_type) errs.task_type = "Choose the kind of work.";
    if (!f.brief_md.trim())
      errs.brief_md = "Describe the work: what the data is, what you need and how it will be used.";
    if (f.volume && !(Number.isInteger(Number(f.volume)) && Number(f.volume) >= 1))
      errs.volume_estimate = "Use a whole number, or leave it empty.";
    if (!f.contact_name.trim()) errs.contact_name = "Enter a contact name.";
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(f.contact_email.trim()))
      errs.contact_email = "Enter a valid email address.";
    if (sample && sample.size > 5 * 1024 * 1024) errs.sample = "The sample file is larger than 5 MB.";
    setErrors(errs);
    setApiError(null);
    if (Object.keys(errs).length) return;
    const body: ProjectRequestInput = {
      title: f.title.trim(),
      task_type: f.task_type as ProjectRequestInput["task_type"],
      brief_md: f.brief_md,
      languages: f.languages,
      volume_estimate: f.volume ? Number(f.volume) : null,
      wanted_by: f.wanted_by || null,
      contact_name: f.contact_name.trim(),
      contact_email: f.contact_email.trim(),
    };
    setBusy(true);
    try {
      let r: ProjectRequest;
      if (sample) {
        const form = new FormData();
        form.append("request", JSON.stringify(body));
        form.append("sample", sample);
        r = await upload<ProjectRequest>("/v1/project-requests", form);
      } else {
        r = await api<ProjectRequest>("POST", "/v1/project-requests", body);
      }
      toast({ tone: "success", title: "Request sent to MezAI" });
      router.push(`/w/requests/${r.id}`);
    } catch (x) {
      const ae = asError(x);
      if (ae.field) setErrors({ [ae.field]: errorMessage(ae) });
      else setApiError(errorMessage(ae));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-6">
      <Breadcrumb items={[{ label: "Project requests", href: "/w/requests" }, { label: "New request" }]} />
      <PageTitle title="Request a project">
        MezAI operations reads every request and replies to the contact below.
      </PageTitle>
      <form onSubmit={submit} noValidate className="grid max-w-3xl gap-6">
        <Card className="grid gap-5 p-5">
          <Field label="Title" required error={errors.title}>
            <Input value={f.title} maxLength={200} onChange={(e) => up({ title: e.target.value })} />
          </Field>
          <ChoiceGroup legend="Kind of work" error={errors.task_type}>
            {requestTypes.map((t) => (
              <Radio
                key={t}
                name="request-type"
                label={taskTypeLabel[t]}
                checked={f.task_type === t}
                onChange={() => up({ task_type: t })}
              />
            ))}
          </ChoiceGroup>
          <Field
            label="Brief"
            required
            hint="What the data is, what you need from it, how the results will be used, and any quality bar you already have."
            error={errors.brief_md}
          >
            <Textarea
              rows={8}
              value={f.brief_md}
              maxLength={20000}
              onChange={(e) => up({ brief_md: e.target.value })}
            />
          </Field>
          <ChoiceGroup legend="Languages" hint="Optional." error={errors.languages}>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
              {languageOptions.map((l) => (
                <Checkbox
                  key={l.value}
                  label={l.label}
                  checked={f.languages.includes(l.value)}
                  onChange={(e) =>
                    up({
                      languages: e.target.checked
                        ? [...f.languages, l.value]
                        : f.languages.filter((x) => x !== l.value),
                    })
                  }
                />
              ))}
            </div>
          </ChoiceGroup>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Rough number of items" error={errors.volume_estimate}>
              <Input inputMode="numeric" value={f.volume} onChange={(e) => up({ volume: e.target.value })} />
            </Field>
            <Field label="Wanted by" error={errors.wanted_by}>
              <Input type="date" value={f.wanted_by} onChange={(e) => up({ wanted_by: e.target.value })} />
            </Field>
          </div>
          <Field
            label="Sample file"
            hint="A few representative rows: .csv, .tsv, .jsonl, .json or .txt, up to 5 MB. Leave out personal data."
            error={errors.sample}
          >
            <Input
              type="file"
              accept={sampleTypes}
              onChange={(e) => setSample(e.target.files?.[0] ?? null)}
            />
          </Field>
        </Card>
        <Card className="grid gap-5 p-5">
          <h2 className="font-display font-semibold">Contact</h2>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Name" required error={errors.contact_name}>
              <Input
                value={f.contact_name}
                maxLength={200}
                onChange={(e) => up({ contact_name: e.target.value })}
              />
            </Field>
            <Field label="Email" required error={errors.contact_email}>
              <Input
                type="email"
                value={f.contact_email}
                onChange={(e) => up({ contact_email: e.target.value })}
              />
            </Field>
          </div>
        </Card>
        {Object.keys(errors).length > 0 && (
          <Alert tone="danger" title="Some fields need attention. They are marked above." />
        )}
        {apiError && <Alert tone="danger" title={apiError} />}
        <div className="flex flex-wrap gap-2">
          <Button type="submit" disabled={busy}>
            {busy ? "Sending…" : "Send request"}
          </Button>
          <Button variant="secondary" onClick={() => router.push("/w/requests")}>
            Cancel
          </Button>
        </div>
      </form>
    </div>
  );
}

function Decline({ request, onDone }: { request: ProjectRequest; onDone: (r: ProjectRequest) => void }) {
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function go() {
    if (!note.trim()) return setError("Write a note. The client sees it.");
    setBusy(true);
    setError(null);
    try {
      onDone(
        await api<ProjectRequest>("POST", `/v1/project-requests/${request.id}/decline`, {
          note: note.trim(),
        }),
      );
      setOpen(false);
    } catch (e) {
      setError(errorMessage(asError(e)));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        Decline
      </Button>
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        title="Decline this request?"
        description="The client sees your note on the request."
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setOpen(false)}>
              Cancel
            </Button>
            <Button variant="danger" size="sm" onClick={go} disabled={busy}>
              Decline
            </Button>
          </>
        }
      >
        <Field label="Note to the client" required error={error ?? undefined}>
          <Textarea rows={4} value={note} maxLength={2000} onChange={(e) => setNote(e.target.value)} />
        </Field>
      </Dialog>
    </>
  );
}

export function RequestDetail() {
  const { id } = useParams<{ id: string }>();
  const can = useCan();
  const router = useRouter();
  const toast = useToast();
  const r = useLoad(() => api<ProjectRequest>("GET", `/v1/project-requests/${id}`), id);
  const [converting, setConverting] = useState(false);

  if (r.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (r.status === "error") return <Alert tone="danger" title={errorMessage(r.error)} />;
  const req = r.data;
  const manage = can("project.manage") && req.status === "submitted";

  const facts: Array<[string, React.ReactNode]> = [
    ["Organisation", req.org_name],
    ["Kind of work", taskTypeLabel[req.task_type]],
    ["Languages", req.languages.map(labelFor).join(", ") || "–"],
    ["Rough number of items", req.volume_estimate?.toLocaleString("en-KE") ?? "–"],
    ["Wanted by", shortDate(req.wanted_by)],
    ["Contact", `${req.contact_name}, ${req.contact_email}`],
    ["Submitted", shortDate(req.created_at)],
  ];

  const initial = (() => {
    const type: TaskType = req.task_type === "other" ? "classification" : req.task_type;
    const f = emptyForm(type, req.org_id);
    f.name = req.title;
    f.target_count = req.volume_estimate ? String(req.volume_estimate) : "";
    f.deadline = req.wanted_by ?? "";
    f.skills = req.languages.map((l) => ({ kind: "language" as const, value: l }));
    return f;
  })();

  return (
    <div className="grid gap-6">
      <Breadcrumb items={[{ label: "Project requests", href: "/w/requests" }, { label: req.title }]} />
      <div className="flex flex-wrap items-start justify-between gap-4">
        <PageTitle title={req.title}>
          <RequestStatus status={req.status} />
        </PageTitle>
        {manage && !converting && (
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => setConverting(true)}>Create project from this</Button>
            <Decline request={req} onDone={(x) => r.set(x)} />
          </div>
        )}
      </div>

      {req.status === "converted" && req.project_id && (
        <Alert tone="success" title="A project was created from this request">
          <Link href={`/w/projects/${req.project_id}`} className="underline underline-offset-4">
            Open the project
          </Link>
        </Alert>
      )}
      {req.status === "declined" && (
        <Alert tone="info" title="MezAI declined this request">
          {req.decision_note}
        </Alert>
      )}

      {converting ? (
        <ProjectFormView
          mode={{
            kind: "convert",
            requestId: req.id,
            orgName: req.org_name,
            taskType: req.task_type === "other" ? undefined : req.task_type,
          }}
          initial={initial}
          onCancel={() => setConverting(false)}
          onSaved={(p) => {
            toast({ tone: "success", title: "Draft project created" });
            router.push(`/w/projects/${p.id}`);
          }}
        />
      ) : (
        <div className="grid gap-6 lg:grid-cols-3">
          <Card className="p-5 text-sm leading-relaxed whitespace-pre-wrap lg:col-span-2">
            <h2 className="mb-2 font-display text-base font-semibold">Brief</h2>
            {req.brief_md}
          </Card>
          <Card className="grid content-start gap-4 p-5">
            <dl className="grid gap-3 text-sm">
              {facts.map(([k, v]) => (
                <div key={k} className="grid gap-0.5">
                  <dt className="text-fg-muted">{k}</dt>
                  <dd className="break-words">{v}</dd>
                </div>
              ))}
            </dl>
            {req.sample_file_name && (
              <a
                href={`/api/v1/project-requests/${req.id}/sample`}
                className="text-sm font-medium break-all underline underline-offset-4"
                download
              >
                Download sample: {req.sample_file_name}
                {req.sample_size != null && ` (${fileSize(req.sample_size)})`}
              </a>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}
