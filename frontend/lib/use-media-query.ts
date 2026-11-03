"use client";

import { useSyncExternalStore } from "react";

/** Whether a media query matches. Server render and hydration assume false. */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (notify) => {
      const mql = matchMedia(query);
      mql.addEventListener("change", notify);
      return () => mql.removeEventListener("change", notify);
    },
    () => matchMedia(query).matches,
    () => false,
  );
}
