"use client";

import type { ReactNode } from "react";
import { m } from "motion/react";
import { reveal, rise, stagger } from "./variants";
import { useReducedMotionGate } from "./use-reduced-motion";

type Props = { children: ReactNode; className?: string; as?: "div" | "section" | "ul" | "li" };

const inView = { once: true, margin: "0px 0px -10% 0px" } as const;

/**
 * Fades and rises into view once. With reduced motion it starts in its final
 * state (initial={false}), so nothing animates.
 */
export function Reveal({ children, className, as = "div" }: Props) {
  const reduce = useReducedMotionGate();
  const Tag = m[as];
  return (
    <Tag
      className={className}
      variants={reveal}
      initial={reduce ? false : "hidden"}
      whileInView="visible"
      viewport={inView}
    >
      {children}
    </Tag>
  );
}

/** A staggered group of RevealItem children; reduced motion skips it like Reveal. */
export function RevealGroup({ children, className, as = "div" }: Props) {
  const reduce = useReducedMotionGate();
  const Tag = m[as];
  return (
    <Tag
      className={className}
      variants={stagger}
      initial={reduce ? false : "hidden"}
      whileInView="visible"
      viewport={inView}
    >
      {children}
    </Tag>
  );
}

/** Child of RevealGroup; the group's initial state (including reduced motion) carries down. */
export function RevealItem({ children, className, as = "div" }: Props) {
  const Tag = m[as];
  return (
    <Tag className={className} variants={rise}>
      {children}
    </Tag>
  );
}
