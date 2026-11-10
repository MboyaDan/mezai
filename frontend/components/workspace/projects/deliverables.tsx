"use client";

import { useState } from "react";
import { Download, PackageCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Agreement, Stat, StatGroup } from "@/components/ui/data";
import { Alert, Card, CardHeader, EmptyState, Skeleton, StatusPill } from "@/components/ui/display";
import { Dialog } from "@/components/ui/dialog";
import { useToast } from "@/components/ui/toast";
import {
  ApiError,
  api,
  type DeliverableFile,
  type DeliverableLink,
  type DeliverableVersion,
} from "@/lib/api/client";
import {
  fileLabel,
  formatBytes,
  headlineAgreement,
  itemsSentence,
  orderedFiles,
  statisticLabel,
} from "@/lib/deliverables";
import { errorMessage } from "@/lib/onboarding";
import { useLoad } from "@/lib/use-load";
import { pct, shortDate } from "../contributors";
import { useCan } from "../workspace";
import { useProject } from "./frame";

const asError = (e: unknown) => (e instanceof ApiError ? e : new ApiError(0, "network"));

/** Deliverables: Ops exports and publishes versions; the client downloads published ones (brief 8.5). */
export function ProjectDeliverables() {
  const { project } = useProject();
  const can = useCan();
  const staff = can("deliverable.publish");
  const toast = useToast();
  const list = useLoad(
    () =>
      api<{ versions: DeliverableVersion[] }>("GET", `/v1/projects/${project.id}/deliverables`).then(
        (r) => r.versions,
      ),
    project.id,
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);
  const [publishing, setPublishing] = useState<number | null>(null);

  async function exportNow() {
    setBusy(true);
    setError(null);
    try {
      const v = await api<DeliverableVersion>("POST", `/v1/projects/${project.id}/deliverables`);
      toast({
        tone: "success",
        title: `Version ${v.version} exported`,
        body: "Check it, then publish it to the client.",
      });
      list.reload();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  async function publish(version: number) {
    setBusy(true);
    setError(null);
    try {
      await api("POST", `/v1/projects/${project.id}/deliverables/${version}/publish`);
      toast({
        tone: "success",
        title: `Version ${version} published`,
        body: "The client can download it now.",
      });
      setPublishing(null);
      list.reload();
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="grid gap-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="grid max-w-2xl gap-1">
          <h2 className="font-display text-xl font-semibold">Deliverables</h2>
          <p className="text-sm text-fg-muted">
            {staff
              ? "Each export holds the final answers as JSONL and CSV with a manifest of counts, gold accuracy, agreement and checksums. Gold and rejected items are never included. The client sees a version only after you publish it."
              : "Final answers as JSONL and CSV, each with a manifest of counts, gold accuracy, agreement and checksums. Download links last five minutes and every download is recorded."}
          </p>
        </div>
        {staff && (
          <Button onClick={exportNow} loading={busy && publishing == null} disabled={busy}>
            Export final items
          </Button>
        )}
      </div>
      {error && publishing == null && <Alert tone="danger" title={errorMessage(error)} />}
      {list.status === "loading" && <Skeleton className="h-64 w-full" />}
      {list.status === "error" && <Alert tone="danger" title={errorMessage(list.error)} />}
      {list.status === "ready" && list.data.length === 0 && (
        <EmptyState title={staff ? "Nothing exported yet" : "No deliverables yet"}>
          {staff
            ? "Export once items are final. You can export again as more items finish; each export is a new version."
            : "The MezAI team will publish the deliverable here once items are final and checked."}
        </EmptyState>
      )}
      {list.status === "ready" &&
        list.data.map((v) => (
          <VersionCard
            key={v.version}
            v={v}
            staff={staff}
            busy={busy}
            onPublish={() => {
              setError(null);
              setPublishing(v.version);
            }}
          />
        ))}
      <Dialog
        open={publishing != null}
        onClose={() => setPublishing(null)}
        title={`Publish version ${publishing ?? ""}?`}
        description="The client's users will see it and can download it. Publishing cannot be undone; export again to deliver a newer version."
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setPublishing(null)} disabled={busy}>
              Cancel
            </Button>
            <Button
              size="sm"
              loading={busy}
              disabled={busy}
              onClick={() => publishing != null && publish(publishing)}
            >
              Publish
            </Button>
          </>
        }
      >
        {error && publishing != null && <Alert tone="danger" title={errorMessage(error)} />}
      </Dialog>
    </div>
  );
}

function VersionCard({
  v,
  staff,
  busy,
  onPublish,
}: {
  v: DeliverableVersion;
  staff: boolean;
  busy: boolean;
  onPublish: () => void;
}) {
  const m = v.manifest;
  const agreement = headlineAgreement(m.agreement);
  return (
    <Card>
      <CardHeader
        title={`Version ${v.version}`}
        meta={`Exported ${shortDate(v.created_at)}${v.published_at ? ` · published ${shortDate(v.published_at)}` : ""}`}
      />
      <div className="grid gap-5 p-5 pt-3">
        <div className="flex flex-wrap items-center gap-3">
          {v.published_at ? (
            <StatusPill status="success">Published</StatusPill>
          ) : (
            <StatusPill status="neutral">Not published</StatusPill>
          )}
          {m.items.complete && <StatusPill status="info">All items final</StatusPill>}
          <p className="text-sm">{itemsSentence(m.items)}</p>
        </div>
        <StatGroup>
          <Stat
            label="Delivered"
            value={m.items.delivered.toLocaleString("en-KE")}
            caption={`${m.items.consensus.toLocaleString("en-KE")} by consensus, ${m.items.adjudicated.toLocaleString("en-KE")} by review`}
          />
          <Stat
            label="Gold accuracy"
            value={pct(m.gold_accuracy.accuracy)}
            caption={
              m.gold_accuracy.results > 0
                ? `${m.gold_accuracy.results.toLocaleString("en-KE")} gold answers`
                : "No gold answers yet"
            }
          />
          <Stat
            label="Review rate"
            value={pct(m.review.review_rate)}
            caption={`${m.review.routed} of ${m.review.completed} items reviewed`}
          />
          <Stat label="Contributors" value={m.contributors.count} caption="Named by ID only" />
        </StatGroup>
        {agreement.length > 0 ? (
          <dl className="grid gap-2 text-sm sm:grid-cols-2">
            {agreement.map((a) => (
              <div
                key={a.statistic}
                className="flex flex-wrap items-baseline justify-between gap-2 border-t border-border pt-2"
              >
                <dt className="text-fg-muted">{statisticLabel[a.statistic] ?? a.statistic}</dt>
                <dd>
                  {a.statistic.endsWith("kappa") ? (
                    <Agreement value={a.value} />
                  ) : a.statistic === "percent" ? (
                    <span className="font-mono tabular">{pct(a.value)}</span>
                  ) : (
                    <span className="font-mono tabular">{a.value.toFixed(2)}</span>
                  )}{" "}
                  <span className="text-fg-muted">over {a.items.toLocaleString("en-KE")} items</span>
                </dd>
              </div>
            ))}
          </dl>
        ) : (
          <p className="text-sm text-fg-muted">
            Agreement needs at least two answers on each item, so none is shown for this version.
          </p>
        )}
        <ul className="grid gap-2" aria-label={`Version ${v.version} files`}>
          {orderedFiles(v.files).map((f) => (
            <FileRow key={`${f.id}-${f.file}`} f={f} staff={staff} />
          ))}
        </ul>
        {staff && !v.published_at && (
          <div className="flex flex-wrap items-center gap-3 border-t border-border pt-4">
            <Button icon={<PackageCheck />} onClick={onPublish} disabled={busy}>
              Publish to client
            </Button>
            <span className="text-sm text-fg-muted">Download and check the files first.</span>
          </div>
        )}
      </div>
    </Card>
  );
}

function FileRow({ f, staff }: { f: DeliverableFile; staff: boolean }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<ApiError | null>(null);

  async function download() {
    setBusy(true);
    setError(null);
    try {
      const link = await api<DeliverableLink>("POST", `/v1/deliverables/${f.id}/links`, { file: f.file });
      window.location.assign(link.url);
    } catch (e) {
      setError(asError(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <li className="grid gap-1 rounded-control border border-border p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="grid min-w-0 gap-0.5">
          <span className="font-medium">{fileLabel(f)}</span>
          <span className="truncate font-mono text-xs text-fg-muted">
            {f.name}
            {f.file === "data" && ` · ${formatBytes(f.bytes)}`}
            {staff &&
              f.file === "data" &&
              ` · ${f.downloads} ${f.downloads === 1 ? "download" : "downloads"}`}
          </span>
          {f.sha256 && (
            <span className="truncate font-mono text-xs text-fg-muted" title={f.sha256}>
              SHA-256 {f.sha256.slice(0, 16)}…
            </span>
          )}
        </div>
        <Button
          variant="secondary"
          size="sm"
          icon={<Download />}
          loading={busy}
          disabled={busy}
          onClick={download}
          aria-label={`Download ${fileLabel(f)}`}
        >
          Download
        </Button>
      </div>
      {error && <Alert tone="danger" title={errorMessage(error)} />}
    </li>
  );
}
