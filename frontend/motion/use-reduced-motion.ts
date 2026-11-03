"use client";

import { useReducedMotion } from "motion/react";

/**
 * True when the visitor asked for reduced motion. The server and the first
 * render before Motion reads the media query return false, so callers must
 * still render full content without animation (see Reveal).
 */
export function useReducedMotionGate(): boolean {
  return useReducedMotion() ?? false;
}
