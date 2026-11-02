"use client";

import type { ComponentProps } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/cn";
import { useField } from "./field";

export const controlClasses =
  "w-full rounded-control border bg-surface text-fg placeholder:text-fg-muted " +
  "transition-[border-color,box-shadow] duration-(--dur-quick) " +
  "hover:border-border-strong/60 disabled:cursor-not-allowed disabled:opacity-60 " +
  "aria-invalid:border-danger-fg";

/** Merges Field context (id, describedby, invalid, required) into a control's props. */
function useFieldProps<P extends { id?: string; "aria-describedby"?: string; required?: boolean }>(props: P) {
  const field = useField();
  if (!field) return props;
  return {
    ...props,
    id: props.id ?? field.id,
    required: props.required ?? field.required,
    "aria-describedby": props["aria-describedby"] ?? field.describedBy,
    "aria-invalid": field.invalid || undefined,
  };
}

export function Input({ className, ...props }: ComponentProps<"input">) {
  const merged = useFieldProps(props);
  return <input className={cn(controlClasses, "min-h-11 border-border px-3", className)} {...merged} />;
}

export function Textarea({ className, rows = 4, ...props }: ComponentProps<"textarea">) {
  const merged = useFieldProps(props);
  return (
    <textarea
      rows={rows}
      className={cn(controlClasses, "min-h-24 border-border px-3 py-2.5 leading-relaxed", className)}
      {...merged}
    />
  );
}

/** Native select: best keyboard, screen reader and mobile behaviour for the pilot. */
export function Select({ className, children, ...props }: ComponentProps<"select">) {
  const merged = useFieldProps(props);
  return (
    <div className="relative">
      <select
        className={cn(controlClasses, "min-h-11 appearance-none border-border pr-10 pl-3", className)}
        {...merged}
      >
        {children}
      </select>
      <ChevronDown
        aria-hidden="true"
        className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-fg-muted"
      />
    </div>
  );
}
