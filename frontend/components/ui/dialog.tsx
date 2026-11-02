"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";
import { IconButton } from "./button";

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

type DialogProps = {
  open: boolean;
  onClose: () => void;
  title: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  className?: string;
};

/**
 * Native <dialog> opened with showModal(): the browser provides the focus
 * trap, inert background, Escape to close and focus return.
 */
function useNativeDialog(open: boolean, onClose: () => void) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (open && !el.open) el.showModal();
    if (!open && el.open) el.close();
  }, [open]);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const handle = (e: Event) => {
      e.preventDefault(); // keep React state as the source of truth
      onClose();
    };
    el.addEventListener("cancel", handle);
    return () => el.removeEventListener("cancel", handle);
  }, [onClose]);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    // The page behind a modal dialog is inert, but browsers let Tab leave for
    // the address bar. Wrap instead, so keyboard focus stays in the dialog.
    const wrap = (e: KeyboardEvent) => {
      if (e.key !== "Tab") return;
      const items = [...el.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((n) => n.offsetParent !== null);
      if (items.length === 0) return;
      const first = items[0];
      const last = items[items.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    };
    el.addEventListener("keydown", wrap);
    return () => el.removeEventListener("keydown", wrap);
  }, []);
  return ref;
}

function Header({
  titleId,
  descId,
  title,
  description,
  onClose,
}: {
  titleId: string;
  descId: string;
  title: ReactNode;
  description?: ReactNode;
  onClose: () => void;
}) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
      <div className="grid gap-1">
        <h2 id={titleId} className="font-display text-xl font-semibold">
          {title}
        </h2>
        {description && (
          <p id={descId} className="text-sm text-fg-muted">
            {description}
          </p>
        )}
      </div>
      <IconButton label="Close" icon={<X />} size="sm" onClick={onClose} className="-mr-2" />
    </div>
  );
}

const surface = "bg-surface text-fg shadow-float backdrop:bg-charcoal-950/55 border border-border";

export function Dialog({ open, onClose, title, description, children, footer, className }: DialogProps) {
  const ref = useNativeDialog(open, onClose);
  const titleId = useId();
  const descId = useId();
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      aria-describedby={description ? descId : undefined}
      onClick={(e) => e.target === e.currentTarget && onClose()}
      className={cn(surface, "dialog-pop m-auto w-[min(32rem,calc(100vw-2rem))] rounded-card p-0", className)}
    >
      <Header titleId={titleId} descId={descId} title={title} description={description} onClose={onClose} />
      {children && <div className="px-5 py-4">{children}</div>}
      {footer && (
        <div className="flex flex-wrap justify-end gap-2 border-t border-border px-5 py-3">{footer}</div>
      )}
    </dialog>
  );
}

/** Side panel for instructions, filters or menus. Same native dialog behaviour. */
export function Drawer({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  side = "right",
  className,
}: DialogProps & { side?: "right" | "left" }) {
  const ref = useNativeDialog(open, onClose);
  const titleId = useId();
  const descId = useId();
  return (
    <dialog
      ref={ref}
      aria-labelledby={titleId}
      aria-describedby={description ? descId : undefined}
      onClick={(e) => e.target === e.currentTarget && onClose()}
      className={cn(
        surface,
        "drawer-slide m-0 h-dvh max-h-dvh w-[min(28rem,100vw)] max-w-none rounded-none p-0",
        side === "right" ? "ml-auto" : "mr-auto",
        className,
      )}
      data-side={side}
    >
      <div className="flex h-full flex-col">
        <Header titleId={titleId} descId={descId} title={title} description={description} onClose={onClose} />
        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="flex flex-wrap justify-end gap-2 border-t border-border px-5 py-3">{footer}</div>
        )}
      </div>
    </dialog>
  );
}
