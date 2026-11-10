"use client";

import { useParams } from "next/navigation";
import { createContext, useContext, useState, type ReactNode } from "react";
import { NavLink } from "@/components/shells/nav-link";
import { Button } from "@/components/ui/button";
import { Dialog } from "@/components/ui/dialog";
import { Alert, Breadcrumb, Skeleton, StatusPill, Tag } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type Project, type ProjectStatus } from "@/lib/api/client";
import { cn } from "@/lib/cn";
import { errorMessage } from "@/lib/onboarding";
import {
  actionsFor,
  missingLabel,
  statusLabel,
  statusTone,
  taskTypeLabel,
  type LifecycleAction,
} from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { shortDate } from "../contributors";
import { useCan } from "../workspace";

export function ProjectStatusPill({ status }: { status: ProjectStatus }) {
  return <StatusPill status={statusTone[status]}>{statusLabel[status]}</StatusPill>;
}

export function TaskTypeTag({ type }: { type: string }) {
  return <Tag>{taskTypeLabel[type as keyof typeof taskTypeLabel] ?? type}</Tag>;
}

type Ctx = { project: Project; reload: () => void; set: (p: Project) => void };
const ProjectContext = createContext<Ctx | null>(null);

/** The project loaded by the surrounding ProjectFrame. */
export function useProject(): Ctx {
  const c = useContext(ProjectContext);
  if (!c) throw new Error("useProject outside ProjectFrame");
  return c;
}

// Dangerous or one-way moves ask for a reason, kept in the audit log.
const needsConfirm: LifecycleAction[] = ["pause", "close"];

function Lifecycle({ project, onChange }: { project: Project; onChange: (p: Project) => void }) {
  const [pending, setPending] = useState<{ action: LifecycleAction; label: string } | null>(null);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ message: string; missing?: string[] } | null>(null);
  const toast = useToast();

  async function run(action: LifecycleAction, label: string, why = "") {
    setBusy(true);
    setError(null);
    try {
      const p = await api<Project>("POST", `/v1/projects/${project.id}/status`, {
        action,
        reason: why.trim() || undefined,
      });
      toast({ tone: "success", title: `${label}: ${statusLabel[p.status].toLowerCase()}` });
      setPending(null);
      setReason("");
      onChange(p);
    } catch (e) {
      const err = e instanceof ApiError ? e : new ApiError(0, "network");
      const missing = Array.isArray(err.body?.missing) ? (err.body.missing as string[]) : undefined;
      setError({ message: errorMessage(err), missing });
    } finally {
      setBusy(false);
    }
  }

  const actions = actionsFor(project.status);
  if (actions.length === 0) return null;
  return (
    <div className="grid justify-items-start gap-3 sm:justify-items-end">
      <div className="flex flex-wrap gap-2">
        {actions.map((a, i) => (
          <Button
            key={a.action}
            size="sm"
            // One lime action per view: the forward move. Close is quiet until confirmed.
            variant={i === 0 && a.action !== "close" ? "primary" : "secondary"}
            disabled={busy}
            onClick={() => (needsConfirm.includes(a.action) ? setPending(a) : run(a.action, a.label))}
          >
            {a.label}
          </Button>
        ))}
      </div>
      {error && !pending && (
        <Alert tone="danger" title={error.message} className="max-w-md">
          {error.missing && (
            <ul className="mt-1 list-disc pl-5">
              {error.missing.map((m) => (
                <li key={m}>{missingLabel[m] ?? m}</li>
              ))}
            </ul>
          )}
        </Alert>
      )}
      <Dialog
        open={pending != null}
        onClose={() => setPending(null)}
        title={pending?.action === "close" ? "Close this project?" : "Pause this project?"}
        description={
          pending?.action === "close"
            ? "Closing is final. Contributors stop receiving its tasks and the settings can no longer change."
            : "Contributors stop receiving new tasks until you resume it."
        }
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setPending(null)}>
              Cancel
            </Button>
            <Button
              variant={pending?.action === "close" ? "danger" : "primary"}
              size="sm"
              disabled={busy}
              onClick={() => pending && run(pending.action, pending.label, reason)}
            >
              {pending?.label}
            </Button>
          </>
        }
      >
        <Field label="Reason" hint="Kept in the audit log.">
          <Textarea rows={3} value={reason} maxLength={1000} onChange={(e) => setReason(e.target.value)} />
        </Field>
        {error && pending && <Alert tone="danger" title={error.message} className="mt-3" />}
      </Dialog>
    </div>
  );
}

function SubNav({ id }: { id: string }) {
  const can = useCan();
  const base = `/w/projects/${id}`;
  const tabs = [
    { href: base, label: "Overview", exact: true, show: true },
    { href: `${base}/items`, label: "Items", show: can("item.import") },
    { href: `${base}/gold`, label: "Gold", show: can("gold.manage") },
    {
      href: `${base}/deliverables`,
      label: "Deliverables",
      show: can("deliverable.publish", "org.deliverable.download"),
    },
    { href: `${base}/settings`, label: "Settings", show: can("project.manage") },
  ].filter((t) => t.show);
  if (tabs.length < 2) return null;
  return (
    <nav aria-label="Project" className="-mx-1 overflow-x-auto border-b border-border">
      <ul className="flex min-w-max gap-1 px-1">
        {tabs.map((t) => (
          <li key={t.href}>
            <NavLink
              href={t.href}
              exact={t.exact}
              className={(active) =>
                cn(
                  "-mb-px inline-flex min-h-11 items-center border-b-2 px-3 text-sm transition-colors",
                  active ? "border-fg font-medium text-fg" : "border-transparent text-fg-muted hover:text-fg",
                )
              }
            >
              {t.label}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  );
}

/** Loads the project once for every tab, with header, status actions and tabs. */
export function ProjectFrame({ children }: { children: ReactNode }) {
  const { id } = useParams<{ id: string }>();
  const can = useCan();
  const p = useLoad(() => api<Project>("GET", `/v1/projects/${id}`), id);

  if (p.status === "loading") return <Skeleton className="h-80 w-full" />;
  if (p.status === "error") return <Alert tone="danger" title={errorMessage(p.error)} />;
  const project = p.data;

  return (
    <ProjectContext.Provider value={{ project, reload: p.reload, set: p.set }}>
      <div className="grid gap-6">
        <Breadcrumb items={[{ label: "Projects", href: "/w/projects" }, { label: project.name }]} />
        <header className="flex flex-wrap items-start justify-between gap-4">
          <div className="grid min-w-0 gap-2">
            <h1 className="font-display text-2xl font-semibold tracking-[-0.01em] break-words">
              {project.name}
            </h1>
            <div className="flex flex-wrap items-center gap-2 text-sm text-fg-muted">
              <ProjectStatusPill status={project.status} />
              <TaskTypeTag type={project.task_type} />
              <span>{project.org_name}</span>
              {project.deadline && <span>· Due {shortDate(project.deadline)}</span>}
            </div>
          </div>
          {can("project.manage") && (
            // Remount when the project changes, so an old "not ready" message clears.
            <Lifecycle
              key={`${project.updated_at}/${project.item_count}/${project.staff?.gold_count}`}
              project={project}
              onChange={p.set}
            />
          )}
        </header>
        <SubNav id={project.id} />
        {children}
      </div>
    </ProjectContext.Provider>
  );
}
