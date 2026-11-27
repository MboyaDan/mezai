// What a project's client users see (Stage 25, brief 6.3, D-167). The API
// refuses hidden panels to client users (403 `not_shared`); these helpers
// only decide what the screens draw.
import type { Project } from "@/lib/api/client";

export type Panel = keyof Project["client_visibility"];

export const visibilityPanels: { key: Panel; label: string; hint: string }[] = [
  { key: "quality", label: "Quality figures", hint: "Gold accuracy, agreement and review rate." },
  { key: "workforce", label: "Workforce", hint: "Contributor counts, skills and per-client codes." },
];

/** Whether the person looking at the project sees the panel. Staff see everything. */
export const sees = (p: Pick<Project, "client_visibility" | "staff">, panel: Panel) =>
  Boolean(p.staff) || p.client_visibility[panel];

/** Staff see a note on panels the client does not see. */
export const hiddenFromClient = (p: Pick<Project, "client_visibility" | "staff">, panel: Panel) =>
  Boolean(p.staff) && !p.client_visibility[panel];
