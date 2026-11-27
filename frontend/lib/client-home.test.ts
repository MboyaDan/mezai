import { describe, expect, it } from "vitest";
import type { Project, ProjectProgress } from "@/lib/api/client";
import { CLIENT_HOME_MAX, clientProjects, clientRows, progressPath } from "./portal";

const project = (id: string, status: Project["status"], created_at: string) =>
  ({ id, status, created_at }) as Project;
const progress = { total: 1 } as ProjectProgress;

describe("client home rows (Stage 26)", () => {
  const list = [
    project("draft", "draft", "2026-11-20"),
    project("closed", "closed", "2026-11-21"),
    project("old-live", "live", "2026-11-01"),
    project("new-live", "live", "2026-11-10"),
    project("paused", "paused", "2026-11-05"),
  ];

  it("orders live, paused, closed, ready, draft, newest first, at most eight", () => {
    expect(clientProjects(list).map((p) => p.id)).toEqual([
      "new-live",
      "old-live",
      "paused",
      "closed",
      "draft",
    ]);
    const many = Array.from({ length: 12 }, (_, i) => project(`p${i}`, "live", `2026-11-${10 + i}`));
    expect(clientProjects(many)).toHaveLength(CLIENT_HOME_MAX);
  });

  it("builds rows from server copies, drafts without progress", () => {
    const out = clientRows(list, () => progress);
    expect(out?.total).toBe(5);
    expect(out?.rows.find((r) => r.project.id === "draft")?.progress).toBeNull();
    expect(out?.rows.find((r) => r.project.id === "paused")?.progress).toBe(progress);
  });

  it("gives up when a shown project's progress is missing, so the browser loads it all", () => {
    expect(clientRows(list, (p) => (p.id === "closed" ? undefined : progress))).toBeNull();
  });

  it("names the progress path the page reads", () => {
    expect(progressPath("abc")).toBe("/v1/projects/abc/progress");
  });
});
