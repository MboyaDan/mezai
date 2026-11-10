"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { Button, IconButton } from "@/components/ui/button";
import { Alert, Card, Skeleton } from "@/components/ui/display";
import { Field } from "@/components/ui/field";
import { Select } from "@/components/ui/input";
import { useToast } from "@/components/ui/toast";
import { ApiError, api, type ProjectReviewers } from "@/lib/api/client";
import { errorMessage } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { useProject } from "./frame";

/** Who may review this project's items (D-111). Ops (review.adjudicate) reviews every project anyway. */
export function Reviewers() {
  const { project } = useProject();
  const toast = useToast();
  const list = useLoad(
    () => api<ProjectReviewers>("GET", `/v1/projects/${project.id}/reviewers`),
    project.id,
  );
  const [pick, setPick] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  async function change(method: "POST" | "DELETE", userId: string, name: string) {
    setBusy(true);
    setError(null);
    try {
      if (method === "POST") await api("POST", `/v1/projects/${project.id}/reviewers`, { user_id: userId });
      else await api("DELETE", `/v1/projects/${project.id}/reviewers/${userId}`);
      toast({
        tone: "success",
        title: method === "POST" ? `${name} can review this project` : `${name} removed`,
      });
      setPick("");
      list.reload();
    } catch (e) {
      setError(e instanceof ApiError ? e : new ApiError(0, "network"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="grid gap-3 p-5">
      <h2 className="font-display font-semibold">Reviewers</h2>
      <p className="text-sm text-fg-muted">
        Reviewers see this project&apos;s review queue. Ops managers see every project.
      </p>
      {list.status === "loading" && <Skeleton className="h-16 w-full" />}
      {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
      {list.status === "ready" && (
        <>
          {list.data.assigned.length === 0 ? (
            <p className="text-sm">No reviewers assigned yet.</p>
          ) : (
            <ul className="grid gap-1 text-sm">
              {list.data.assigned.map((r) => (
                <li key={r.id} className="flex items-center justify-between gap-2">
                  <span className="min-w-0 truncate">{r.display_name}</span>
                  <IconButton
                    size="sm"
                    variant="ghost"
                    label={`Remove ${r.display_name}`}
                    icon={<X />}
                    disabled={busy}
                    onClick={() => change("DELETE", r.id, r.display_name)}
                  />
                </li>
              ))}
            </ul>
          )}
          {list.data.candidates.length > 0 && (
            <div className="flex flex-wrap items-end gap-2">
              <Field label="Add a reviewer" className="min-w-0 flex-1">
                <Select value={pick} onChange={(e) => setPick(e.target.value)}>
                  <option value="">Choose…</option>
                  {list.data.candidates.map((c) => (
                    <option key={c.id} value={c.id}>
                      {c.display_name}
                    </option>
                  ))}
                </Select>
              </Field>
              <Button
                size="sm"
                variant="secondary"
                disabled={!pick || busy}
                onClick={() => {
                  const c = list.data.candidates.find((x) => x.id === pick);
                  if (c) change("POST", c.id, c.display_name);
                }}
              >
                Add
              </Button>
            </div>
          )}
        </>
      )}
      {error && <Alert tone="danger" title={errorMessage(error)} />}
    </Card>
  );
}
