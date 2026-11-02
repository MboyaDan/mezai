"use client";

import { cloneElement, useEffect, useId, useState, type ReactElement } from "react";
import { cn } from "@/lib/cn";

type TooltipProps = {
  content: string;
  /** A single focusable element (button, link). Its accessible name must not depend on the tooltip. */
  children: ReactElement<{ "aria-describedby"?: string }>;
  side?: "top" | "bottom";
};

/**
 * Supplementary text on hover and keyboard focus (WCAG 1.4.13): it stays
 * while hovered, dismisses with Escape, and is linked via aria-describedby.
 * Never put essential information only in a tooltip.
 */
export function Tooltip({ content, children, side = "top" }: TooltipProps) {
  const id = useId();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  return (
    <span
      className="relative inline-flex"
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onFocus={() => setOpen(true)}
      onBlur={() => setOpen(false)}
    >
      {cloneElement(children, { "aria-describedby": id })}
      <span
        id={id}
        role="tooltip"
        data-theme="dark"
        className={cn(
          "pointer-events-none absolute left-1/2 z-40 w-max max-w-64 -translate-x-1/2 rounded-control bg-surface-raised px-2.5 py-1.5 text-xs leading-snug text-fg shadow-float",
          "transition-opacity duration-(--dur-quick)",
          side === "top" ? "bottom-full mb-2" : "top-full mt-2",
          open ? "opacity-100" : "invisible opacity-0",
        )}
      >
        {content}
      </span>
    </span>
  );
}
