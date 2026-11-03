"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { CircleAlert, CircleCheck, Info, X } from "lucide-react";
import { cn } from "@/lib/cn";

type ToastTone = "info" | "success" | "danger";
type Toast = { id: number; tone: ToastTone; title: string; body?: string };

const ToastContext = createContext<((t: Omit<Toast, "id">) => void) | null>(null);

export function useToast() {
  const push = useContext(ToastContext);
  if (!push) throw new Error("useToast must be used inside <ToastProvider>");
  return push;
}

const icons = { info: Info, success: CircleCheck, danger: CircleAlert };
// Toasts sit on a dark surface: lime and ochre keep icon contrast there.
const iconColour = { info: "text-fg", success: "text-lime", danger: "text-ochre" };

/**
 * Toasts announce through a polite live region (danger is assertive).
 * They auto-dismiss after 6 s, pausing while hovered or focused.
 */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(1);

  const dismiss = useCallback((id: number) => setToasts((ts) => ts.filter((t) => t.id !== id)), []);
  const push = useCallback((t: Omit<Toast, "id">) => {
    const id = nextId.current++;
    setToasts((ts) => [...ts.slice(-3), { ...t, id }]);
  }, []);

  return (
    <ToastContext.Provider value={push}>
      {children}
      <div
        aria-live="polite"
        className="pointer-events-none fixed inset-x-4 bottom-4 z-50 flex flex-col items-end gap-2 sm:inset-x-auto sm:right-6 sm:bottom-6"
      >
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onDismiss={() => dismiss(t.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastItem({ toast, onDismiss }: { toast: Toast; onDismiss: () => void }) {
  const [paused, setPaused] = useState(false);
  useEffect(() => {
    if (paused) return;
    const timer = setTimeout(onDismiss, 6000);
    return () => clearTimeout(timer);
  }, [paused, onDismiss]);

  const Icon = icons[toast.tone];
  return (
    <div
      role={toast.tone === "danger" ? "alert" : "status"}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
      data-theme="dark"
      className="toast-enter pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-card border border-border bg-surface-raised p-4 shadow-float"
    >
      <Icon aria-hidden="true" className={cn("mt-0.5 size-5 shrink-0", iconColour[toast.tone])} />
      <div className="grid flex-1 gap-0.5">
        <p className="font-semibold">{toast.title}</p>
        {toast.body && <p className="text-sm text-fg-muted">{toast.body}</p>}
      </div>
      <button
        type="button"
        onClick={onDismiss}
        aria-label="Dismiss notification"
        className="-m-1 rounded-control p-1 text-fg-muted hover:text-fg"
      >
        <X aria-hidden="true" className="size-4" />
      </button>
    </div>
  );
}
