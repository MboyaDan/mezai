"use client";

import { useId, useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import { cn } from "@/lib/cn";

type Tab = { id: string; label: ReactNode; content: ReactNode };

/**
 * WAI-ARIA tabs with automatic activation: arrow keys move and select,
 * Home/End jump, only the active tab is in the tab order.
 */
export function Tabs({
  tabs,
  defaultTab,
  label,
  className,
}: {
  tabs: Tab[];
  defaultTab?: string;
  label: string;
  className?: string;
}) {
  const base = useId();
  const [active, setActive] = useState(defaultTab ?? tabs[0]?.id);
  const refs = useRef<Array<HTMLButtonElement | null>>([]);

  function onKeyDown(e: KeyboardEvent, i: number) {
    const last = tabs.length - 1;
    const next = {
      ArrowRight: i === last ? 0 : i + 1,
      ArrowLeft: i === 0 ? last : i - 1,
      Home: 0,
      End: last,
    }[e.key];
    if (next === undefined) return;
    e.preventDefault();
    setActive(tabs[next].id);
    refs.current[next]?.focus();
  }

  return (
    <div className={cn("min-w-0", className)}>
      <div role="tablist" aria-label={label} className="flex gap-1 overflow-x-auto border-b border-border">
        {tabs.map((t, i) => {
          const selected = t.id === active;
          return (
            <button
              key={t.id}
              ref={(el) => {
                refs.current[i] = el;
              }}
              id={`${base}-tab-${t.id}`}
              role="tab"
              type="button"
              aria-selected={selected}
              aria-controls={`${base}-panel-${t.id}`}
              tabIndex={selected ? 0 : -1}
              onClick={() => setActive(t.id)}
              onKeyDown={(e) => onKeyDown(e, i)}
              className={cn(
                "-mb-px min-h-11 border-b-2 px-3 text-[0.9375rem] font-medium whitespace-nowrap transition-colors duration-(--dur-quick)",
                selected ? "border-fg text-fg" : "border-transparent text-fg-muted hover:text-fg",
              )}
            >
              {t.label}
            </button>
          );
        })}
      </div>
      {tabs.map((t) => (
        <div
          key={t.id}
          id={`${base}-panel-${t.id}`}
          role="tabpanel"
          aria-labelledby={`${base}-tab-${t.id}`}
          hidden={t.id !== active}
          tabIndex={0}
          className="pt-4"
        >
          {t.content}
        </div>
      ))}
    </div>
  );
}
