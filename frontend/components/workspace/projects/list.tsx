"use client";

import Link from "next/link";
import { useState } from "react";
import { ButtonLink } from "@/components/ui/button";
import { Alert, EmptyState, Skeleton } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/input";
import { Table } from "@/components/ui/table";
import { api, type Project, type ProjectStatus } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { statusLabel } from "@/lib/projects";
import { useUser } from "@/lib/session";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "../contributors";
import { PageTitle, useCan } from "../workspace";
import { ProjectStatusPill, TaskTypeTag } from "./frame";

const statuses = Object.keys(statusLabel) as ProjectStatus[];

export function Projects() {
  const can = useCan();
  const user = useUser();
  const staff = user.kind === "staff";
  const [status, setStatus] = useState<ProjectStatus | "">("");
  const list = useLoad(
    () =>
      api<{ projects: Project[] }>("GET", `/v1/projects${status ? `?status=${status}` : ""}`).then(
        (r) => r.projects,
      ),
    status,
  );

  return (
    <>
      <div className="mb-6 flex flex-wrap items-start justify-between gap-4 [&>header]:mb-0">
        <PageTitle title="Projects">
          {staff ? "Every client project, from draft to closed." : "Your organisation's projects with MezAI."}
        </PageTitle>
        {can("project.manage") && <ButtonLink href="/w/projects/new">New project</ButtonLink>}
        {!staff && can("org.project.request") && (
          <ButtonLink href="/w/requests/new" variant="secondary">
            Request a project
          </ButtonLink>
        )}
      </div>
      <div className="grid gap-4">
        <Field label="Status" className="max-w-56">
          <Select value={status} onChange={(e) => setStatus(e.target.value as ProjectStatus | "")}>
            <option value="">All statuses</option>
            {statuses.map((s) => (
              <option key={s} value={s}>
                {statusLabel[s]}
              </option>
            ))}
          </Select>
        </Field>
        {list.status === "loading" && <Skeleton className="h-40 w-full" />}
        {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
        {list.status === "ready" && (
          <Table
            caption="Projects, most recently changed first"
            hideCaption
            rows={list.data}
            rowKey={(p) => p.id}
            empty={
              <EmptyState title={status ? "No projects with this status" : "No projects yet"}>
                {staff
                  ? "Create a project, or convert a client's project request."
                  : "When MezAI sets up a project for your organisation it appears here."}
              </EmptyState>
            }
            columns={[
              {
                key: "name",
                header: "Project",
                cell: (p) => (
                  <Link
                    href={`/w/projects/${p.id}`}
                    className="inline-block min-w-48 font-medium underline underline-offset-4"
                  >
                    {p.name}
                  </Link>
                ),
                sortValue: (p) => p.name.toLowerCase(),
              },
              ...(staff
                ? [
                    {
                      key: "org",
                      header: "Organisation",
                      cell: (p: Project) => p.org_name,
                      sortValue: (p: Project) => p.org_name,
                    },
                  ]
                : []),
              { key: "type", header: "Task type", cell: (p) => <TaskTypeTag type={p.task_type} /> },
              {
                key: "status",
                header: "Status",
                cell: (p) => <ProjectStatusPill status={p.status} />,
                sortValue: (p) => statuses.indexOf(p.status),
              },
              {
                key: "items",
                header: "Items / target",
                numeric: true,
                cell: (p) =>
                  `${p.item_count.toLocaleString("en-KE")} / ${p.target_count?.toLocaleString("en-KE") ?? "–"}`,
                sortValue: (p) => p.item_count,
              },
              {
                key: "deadline",
                header: "Deadline",
                cell: (p) => shortDate(p.deadline),
                sortValue: (p) => p.deadline ?? "9999",
              },
            ]}
          />
        )}
      </div>
    </>
  );
}
