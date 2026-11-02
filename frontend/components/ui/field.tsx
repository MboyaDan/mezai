"use client";

import { createContext, useContext, useId, type ReactNode } from "react";
import { CircleAlert } from "lucide-react";
import { cn } from "@/lib/cn";

type FieldContextValue = { id: string; describedBy?: string; invalid: boolean; required: boolean };

const FieldContext = createContext<FieldContextValue | null>(null);

/** Read by Input, Textarea and Select so a Field wires label, hint and error automatically. */
export function useField() {
  return useContext(FieldContext);
}

type FieldProps = {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  /** Visually hide the label (it stays available to screen readers). */
  hideLabel?: boolean;
  className?: string;
  children: ReactNode;
};

export function Field({ label, hint, error, required = false, hideLabel, className, children }: FieldProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [errorId, hintId].filter(Boolean).join(" ") || undefined;

  return (
    <FieldContext.Provider value={{ id, describedBy, invalid: Boolean(error), required }}>
      <div className={cn("grid gap-1.5", className)}>
        <label htmlFor={id} className={cn("text-sm font-medium text-fg", hideLabel && "sr-only")}>
          {label}
          {required ? (
            <span className="ml-1 text-fg-muted" aria-hidden="true">
              *
            </span>
          ) : (
            <span className="ml-1.5 text-xs font-normal text-fg-muted">(optional)</span>
          )}
        </label>
        {hint && (
          <p id={hintId} className="text-sm text-fg-muted">
            {hint}
          </p>
        )}
        {children}
        {error && (
          <p id={errorId} className="flex items-start gap-1.5 text-sm font-medium text-danger-fg">
            <CircleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <span>{error}</span>
          </p>
        )}
      </div>
    </FieldContext.Provider>
  );
}
