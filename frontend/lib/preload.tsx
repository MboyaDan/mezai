"use client";

// Data a server component already read for this page (lib/api/server.ts),
// handed to the page's hooks so they start ready instead of fetching after
// hydration (Stage 23, D-165). Nested providers add to their parent's map.

import { createContext, useContext, useMemo, type ReactNode } from "react";

type Preloaded = Record<string, unknown>;
const Ctx = createContext<Preloaded>({});

export function PreloadProvider({ data, children }: { data: Preloaded; children: ReactNode }) {
  const parent = useContext(Ctx);
  const value = useMemo(() => ({ ...parent, ...data }), [parent, data]);
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

/** The server's copy of an API path's answer, or undefined if it has none. */
export function usePreloaded<T>(path: string): T | undefined {
  return useContext(Ctx)[path] as T | undefined;
}

/** Every server copy the page holds, for paths built from data (Stage 26). */
export function usePreloadedMap(): Readonly<Preloaded> {
  return useContext(Ctx);
}
