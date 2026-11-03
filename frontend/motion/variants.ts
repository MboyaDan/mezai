import type { Transition, Variants } from "motion/react";
import { dur, ease, staggerMax, staggerStep } from "./tokens";

/** Section reveal: fade in with a 16 px rise (pattern 1). */
export const reveal: Variants = {
  hidden: { opacity: 0, y: 16 },
  visible: { opacity: 1, y: 0, transition: { duration: dur.slow, ease: ease.out } },
};

/** Group item: shorter rise. Timing comes from the `stagger` parent. */
export const rise: Variants = {
  hidden: { opacity: 0, y: 12 },
  visible: { opacity: 1, y: 0, transition: { duration: dur.slow, ease: ease.out } },
};

/**
 * Group container: children enter 60 ms apart. Past the eighth child the
 * stagger stops growing, so long lists never trail (DESIGN.md pattern 1).
 */
export const stagger: Variants = {
  hidden: {},
  visible: {
    transition: {
      delayChildren: (i: number) => Math.min(i, staggerMax - 1) * staggerStep,
    },
  },
};

/** Press feedback (pattern 6): spread onto an `m.*` element. */
export const press: { whileTap: { scale: number }; transition: Transition } = {
  whileTap: { scale: 0.98 },
  transition: { duration: dur.instant, ease: ease.out },
};
