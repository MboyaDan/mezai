"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError } from "@/lib/api/client";

type Loaded<T> =
  | { status: "loading"; reload: () => void }
  | { status: "error"; error: ApiError; reload: () => void }
  | { status: "ready"; data: T; reload: () => void; set: (d: T) => void };

/**
 * Loads data on mount (and when `key` changes); `reload` fetches again.
 * With `initial` (data the server already read, lib/preload.tsx) it starts
 * ready and skips the first fetch; a later key change or reload fetches.
 */
export function useLoad<T>(load: () => Promise<T>, key = "", initial?: T): Loaded<T> {
  const [state, setState] = useState<{ s: "loading" } | { s: "error"; e: ApiError } | { s: "ready"; d: T }>(
    () => (initial === undefined ? { s: "loading" } : { s: "ready", d: initial }),
  );
  const [tick, setTick] = useState(0);
  const skip = useRef(initial !== undefined);

  useEffect(() => {
    if (skip.current) {
      skip.current = false;
      return;
    }
    let live = true;
    load().then(
      (d) => live && setState({ s: "ready", d }),
      (e) => live && setState({ s: "error", e: e instanceof ApiError ? e : new ApiError(0, "network") }),
    );
    return () => {
      live = false;
    };
    // `load` is usually an inline function; `key` and `tick` decide when to refetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, tick]);

  const reload = useCallback(() => setTick((t) => t + 1), []);
  const set = useCallback((d: T) => setState({ s: "ready", d }), []);
  if (state.s === "loading") return { status: "loading", reload };
  if (state.s === "error") return { status: "error", error: state.e, reload };
  return { status: "ready", data: state.d, reload, set };
}
