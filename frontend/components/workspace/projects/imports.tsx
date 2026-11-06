"use client";

import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, Card, Skeleton, StatusPill } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, upload, type ImportJob, type TaskTemplate } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { fileSize, headerLine } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "../contributors";
import { useProject } from "./frame";

type Kind = "items" | "gold";

const statusText: Record<ImportJob["status"], { label: string; tone: "success" | "danger" | "info" }> = {
  validated: { label: "Checked, not imported", tone: "info" },
  failed: { label: "Has errors", tone: "danger" },
  committed: { label: "Imported", tone: "success" },
};

/** Past uploads for a project: the dataset versions. */
export function ImportHistory({ jobs }: { jobs: ImportJob[] }) {
  return (
    <Table
      caption="Uploaded files, newest first"
      hideCaption
      rows={jobs}
      rowKey={(j) => j.id}
      empty="No files uploaded yet."
      columns={[
        {
          key: "file",
          header: "File",
          cell: (j) => <span className="break-all">{j.file_name}</span>,
          sortValue: (j) => j.file_name,
        },
        { key: "size", header: "Size", numeric: true, cell: (j) => fileSize(j.size_bytes) },
        {
          key: "rows",
          header: "Valid / rows",
          numeric: true,
          cell: (j) => `${j.valid_rows.toLocaleString("en-KE")} / ${j.total_rows.toLocaleString("en-KE")}`,
        },
        {
          key: "status",
          header: "Status",
          cell: (j) => (
            <StatusPill status={statusText[j.status].tone}>{statusText[j.status].label}</StatusPill>
          ),
        },
        {
          key: "at",
          header: "Uploaded",
          cell: (j) => shortDate(j.committed_at ?? j.created_at),
          sortValue: (j) => j.created_at,
        },
      ]}
    />
  );
}

function Report({ job }: { job: ImportJob }) {
  const errors = job.errors ?? [];
  return (
    <div className="grid gap-4">
      <dl className="grid grid-cols-3 gap-3">
        {[
          ["Rows", job.total_rows],
          ["Valid", job.valid_rows],
          ["With errors", job.error_rows],
        ].map(([k, v]) => (
          <div key={k} className="rounded-card border border-border p-3">
            <dt className="text-xs text-fg-muted">{k}</dt>
            <dd className="font-mono text-xl tabular">{Number(v).toLocaleString("en-KE")}</dd>
          </div>
        ))}
      </dl>
      {errors.length > 0 && (
        <>
          <Table
            caption={`Problems in ${job.file_name}`}
            rows={errors}
            rowKey={(e) => `${e.row}/${e.field ?? ""}/${e.message}`}
            empty=""
            columns={[
              {
                key: "row",
                header: job.format === "csv" ? "Row" : "Line",
                numeric: true,
                cell: (e) => e.row,
              },
              {
                key: "field",
                header: "Field",
                cell: (e) => <span className="font-mono text-xs">{e.field || "–"}</span>,
              },
              { key: "msg", header: "Problem", cell: (e) => e.message },
            ]}
          />
          {job.errors_truncated && (
            <p className="text-sm text-fg-muted">
              Showing the first {errors.length.toLocaleString("en-KE")} problems. The counts above cover the
              whole file.
            </p>
          )}
        </>
      )}
    </div>
  );
}

/**
 * Upload → check (dry run, nothing imported) → report → import all rows.
 * The API re-reads and re-checks the stored file on commit (D-079).
 */
export function ImportPanel({ kind, onImported }: { kind: Kind; onImported: () => void }) {
  const { project } = useProject();
  const templates = useLoad(() =>
    api<{ templates: TaskTemplate[] }>("GET", "/v1/task-templates").then((r) => r.templates),
  );
  const fileRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [job, setJob] = useState<ImportJob | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const toast = useToast();
  const closed = project.status === "closed";

  const template =
    templates.status === "ready" ? templates.data.find((t) => t.task_type === project.task_type) : undefined;
  const columns = template ? (kind === "gold" ? template.gold_columns : template.item_columns) : [];

  function reset() {
    setJob(null);
    setFile(null);
    setError(null);
    if (fileRef.current) fileRef.current.value = "";
  }

  async function check() {
    if (!file) return setError("Choose a .csv or .jsonl file first.");
    if (file.size > 10 * 1024 * 1024)
      return setError("The file is larger than 10 MB. Split it and upload each part.");
    setBusy(true);
    setError(null);
    const form = new FormData();
    form.append("kind", kind);
    form.append("file", file);
    try {
      setJob(await upload<ImportJob>(`/v1/projects/${project.id}/imports`, form));
    } catch (e) {
      setError(errorMessage(e instanceof ApiError ? e : new ApiError(0, "network")));
    } finally {
      setBusy(false);
    }
  }

  async function commit() {
    if (!job) return;
    setBusy(true);
    setError(null);
    try {
      const done = await api<ImportJob>("POST", `/v1/imports/${job.id}/commit`);
      toast({
        tone: "success",
        title: `Imported ${done.valid_rows.toLocaleString("en-KE")} ${kind === "gold" ? "gold items" : "items"}`,
      });
      reset();
      onImported();
    } catch (e) {
      setError(errorMessage(e instanceof ApiError ? e : new ApiError(0, "network")));
    } finally {
      setBusy(false);
    }
  }

  if (closed) return <Alert tone="info" title="This project is closed. No more files can be imported." />;

  return (
    <Card className="grid gap-5 p-5">
      <div className="grid gap-1">
        <h2 className="font-display font-semibold">
          {kind === "gold" ? "Import gold items" : "Import items"}
        </h2>
        <p className="text-sm text-fg-muted">
          Upload a CSV with a header row, or JSONL with one object per line (UTF-8, up to 10 MB and 20,000
          rows). We check every row first; nothing is imported until you confirm, and a file with any error
          imports nothing.
        </p>
      </div>
      {templates.status === "loading" && <Skeleton className="h-10 w-full" />}
      {columns.length > 0 && (
        <div className="grid gap-1.5 text-sm">
          <p className="text-fg-muted">Columns for {template?.name}</p>
          <code className="block overflow-x-auto rounded-control bg-sunken px-3 py-2 font-mono text-xs whitespace-pre">
            {headerLine(columns)}
          </code>
          <p className="text-fg-muted">
            <span className="font-mono">external_id</span> is optional: your own ID for the row, returned in
            the deliverable.
            {kind === "gold" && project.task_type === "classification" && (
              <> Separate several expected labels with a vertical bar ( | ).</>
            )}
          </p>
        </div>
      )}

      {!job && (
        <div className="flex flex-wrap items-end gap-3">
          <Field label="File" required className="min-w-0 flex-1 basis-64">
            <Input
              ref={fileRef}
              type="file"
              accept=".csv,.jsonl,text/csv,application/x-ndjson"
              onChange={(e) => {
                setFile(e.target.files?.[0] ?? null);
                setError(null);
              }}
            />
          </Field>
          <Button variant="primary" onClick={check} disabled={busy || !file}>
            {busy ? "Checking…" : "Check file"}
          </Button>
        </div>
      )}

      {job && (
        <div className="grid gap-4" aria-live="polite">
          {job.status === "validated" ? (
            <Alert tone="success" title={`${job.file_name}: every row is valid`}>
              Nothing has been imported yet.
            </Alert>
          ) : (
            <Alert tone="danger" title={`${job.file_name}: fix the problems below and upload again`}>
              Nothing was imported.
            </Alert>
          )}
          <Report job={job} />
          <div className="flex flex-wrap gap-2">
            {job.status === "validated" && (
              <Button variant="primary" onClick={commit} disabled={busy}>
                {busy ? "Importing…" : `Import ${job.valid_rows.toLocaleString("en-KE")} rows`}
              </Button>
            )}
            <Button variant="secondary" onClick={reset} disabled={busy}>
              {job.status === "validated" ? "Cancel" : "Upload another file"}
            </Button>
          </div>
        </div>
      )}
      {error && <Alert tone="danger" title={error} />}
    </Card>
  );
}

/** Items tab: the import wizard and every item file uploaded so far. */
export function ProjectItems() {
  const { project, reload } = useProject();
  const [tick, setTick] = useState(0);
  const list = useLoad(
    () =>
      api<{ imports: ImportJob[] }>("GET", `/v1/projects/${project.id}/imports`).then((r) =>
        r.imports.filter((j) => j.kind === "items"),
      ),
    `${project.id}/${tick}`,
  );
  return (
    <div className="grid gap-6">
      <ImportPanel
        kind="items"
        onImported={() => {
          setTick((t) => t + 1);
          reload();
        }}
      />
      <section aria-labelledby="hist-h" className="grid gap-3">
        <h2 id="hist-h" className="font-display font-semibold">
          Item files
        </h2>
        {list.status === "loading" && <Skeleton className="h-24 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" && <ImportHistory jobs={list.data} />}
      </section>
    </div>
  );
}
