"use client";

import { useState } from "react";
import { Switch } from "@/components/ui/choice";
import { Card } from "@/components/ui/display";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type Project } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { visibilityPanels, type Panel } from "@/lib/visibility";
import { useProject } from "./frame";

/**
 * What the client's users see on this project (brief 6.3, D-167), for Ops
 * with `project.manage`. Each switch saves at once and is audited.
 */
export function ClientVisibilityCard() {
  const { project: p, set } = useProject();
  const toast = useToast();
  const [busy, setBusy] = useState<Panel | null>(null);

  async function toggle(panel: Panel, on: boolean) {
    setBusy(panel);
    try {
      const next = await api<Project>("PUT", `/v1/projects/${p.id}/client-visibility`, {
        ...p.client_visibility,
        [panel]: on,
      });
      set(next);
      const name = visibilityPanels.find((x) => x.key === panel)!.label;
      toast({
        tone: "success",
        title: on ? `${name} shared with the client` : `${name} hidden from the client`,
      });
    } catch (e) {
      toast({
        tone: "danger",
        title: e instanceof ApiError ? errorMessage(e) : "Could not save. Try again.",
      });
    } finally {
      setBusy(null);
    }
  }

  return (
    <section aria-labelledby="client-sees-h" className="grid gap-3">
      <h2 id="client-sees-h" className="font-display font-semibold">
        What the client sees
      </h2>
      <Card className="grid gap-4 p-5 text-sm">
        {visibilityPanels.map((x) => (
          <div key={x.key} className="grid gap-1">
            <Switch
              label={x.label}
              checked={p.client_visibility[x.key]}
              disabled={busy !== null}
              onCheckedChange={(on) => toggle(x.key, on)}
            />
            <p className="pl-14 text-fg-muted">{x.hint}</p>
          </div>
        ))}
        <p className="border-t border-border pt-4 text-fg-muted">
          Progress, datasets, deliverables and data retention are always shown. Delivered files keep their
          manifest with gold accuracy and agreement.
        </p>
      </Card>
    </section>
  );
}
