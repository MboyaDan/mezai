"use client";

import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Breadcrumb, Skeleton } from "@/components/ui/display";
import { TextLink } from "@/components/ui/text-link";
import { useToast } from "@/components/ui/toast";
import { api, type Organisation } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { emptyForm, formFromProject } from "@/lib/projects";
import { useLoad } from "@/lib/use-load";
import { PageTitle, useCan } from "../workspace";
import { useProject } from "./frame";
import { ProjectFormView } from "./form";

export function NewProject() {
  const router = useRouter();
  const params = useSearchParams();
  const can = useCan();
  const toast = useToast();
  const orgs = useLoad(() =>
    api<{ organisations: Organisation[] }>("GET", "/v1/organisations").then((r) => r.organisations),
  );
  return (
    <div className="grid gap-6">
      <Breadcrumb items={[{ label: "Projects", href: "/w/projects" }, { label: "New project" }]} />
      <PageTitle title="New project">
        Starts as a draft. Nothing reaches contributors until you import items and publish.
      </PageTitle>
      {orgs.status === "loading" && <Skeleton className="h-96 w-full max-w-3xl" />}
      {orgs.status === "error" && <Alert tone="danger" title={errorMessage(orgs.error)} />}
      {orgs.status === "ready" &&
        (orgs.data.some((o) => o.status === "active") ? (
          <ProjectFormView
            mode={{ kind: "create", organisations: orgs.data }}
            initial={emptyForm("classification", params.get("org") ?? "")}
            onCancel={() => router.push("/w/projects")}
            onSaved={(p) => {
              toast({ tone: "success", title: "Draft project created" });
              router.push(`/w/projects/${p.id}`);
            }}
          />
        ) : (
          <Alert tone="info" title="Create the client organisation first">
            {can("org.manage") ? (
              <TextLink href="/w/organisations">Go to organisations</TextLink>
            ) : (
              "Ask someone who manages organisations to add it."
            )}
          </Alert>
        ))}
    </div>
  );
}

export function ProjectSettings() {
  const { project, set } = useProject();
  const toast = useToast();
  return (
    <ProjectFormView
      // Remount after a save so the form shows what the API stored.
      key={project.updated_at}
      mode={{ kind: "edit", project }}
      initial={formFromProject(project)}
      onSaved={(p) => {
        toast({ tone: "success", title: "Settings saved" });
        set(p);
      }}
    />
  );
}
