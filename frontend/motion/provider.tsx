"use client";

import type { ReactNode } from "react";
import { LazyMotion, MotionConfig } from "motion/react";

const loadFeatures = () => import("./features").then((mod) => mod.default);

/**
 * Wraps the app once. `strict` makes any `motion.*` import throw, so code
 * uses the lighter `m.*` components. reducedMotion="user" is a backstop; the
 * components here also use useReducedMotionGate to skip animation entirely.
 */
export function MotionProvider({ children }: { children: ReactNode }) {
  return (
    <LazyMotion features={loadFeatures} strict>
      <MotionConfig reducedMotion="user">{children}</MotionConfig>
    </LazyMotion>
  );
}
