"use client";

import { useId, type ComponentProps, type ReactNode } from "react";
import { cn } from "@/lib/cn";

const box =
  "size-5 shrink-0 rounded-control border border-border-strong/70 bg-surface accent-[var(--color-lime-ink)] " +
  "disabled:cursor-not-allowed disabled:opacity-60";

type ChoiceProps = Omit<ComponentProps<"input">, "type"> & { label: ReactNode; hint?: ReactNode };

function Choice({ type, label, hint, className, id, ...rest }: ChoiceProps & { type: "checkbox" | "radio" }) {
  const autoId = useId();
  const inputId = id ?? autoId;
  const hintId = hint ? `${inputId}-hint` : undefined;
  return (
    <div className={cn("flex items-start gap-3", className)}>
      <input
        id={inputId}
        type={type}
        aria-describedby={hintId}
        className={cn(box, type === "radio" && "rounded-full", "mt-0.5")}
        {...rest}
      />
      <div className="grid gap-0.5">
        <label htmlFor={inputId} className="text-[0.9375rem] leading-snug text-fg">
          {label}
        </label>
        {hint && (
          <span id={hintId} className="text-sm text-fg-muted">
            {hint}
          </span>
        )}
      </div>
    </div>
  );
}

/** Native checkbox, tinted with lime-ink (passes contrast on light surfaces). */
export function Checkbox(props: ChoiceProps) {
  return <Choice type="checkbox" {...props} />;
}

export function Radio(props: ChoiceProps) {
  return <Choice type="radio" {...props} />;
}

type RadioGroupProps = {
  legend: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  className?: string;
  children: ReactNode;
};

/** Fieldset with a legend; group Radios (sharing one `name`) or related Checkboxes. */
export function ChoiceGroup({ legend, hint, error, className, children }: RadioGroupProps) {
  return (
    <fieldset className={cn("grid gap-3", className)} aria-invalid={error ? true : undefined}>
      <legend className="mb-1 text-sm font-medium text-fg">{legend}</legend>
      {hint && <p className="-mt-2 text-sm text-fg-muted">{hint}</p>}
      {children}
      {error && <p className="text-sm font-medium text-danger-fg">{error}</p>}
    </fieldset>
  );
}

type SwitchProps = Omit<ComponentProps<"button">, "onChange"> & {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  label: ReactNode;
};

/** On/off setting that applies immediately. Use Checkbox for choices submitted with a form. */
export function Switch({ checked, onCheckedChange, label, className, id, ...rest }: SwitchProps) {
  const autoId = useId();
  const switchId = id ?? autoId;
  return (
    <div className={cn("flex items-center gap-3", className)}>
      <button
        id={switchId}
        type="button"
        role="switch"
        aria-checked={checked}
        onClick={() => onCheckedChange(!checked)}
        className={cn(
          "relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border transition-colors duration-(--dur-quick)",
          checked ? "border-charcoal-950 bg-lime" : "border-border-strong/40 bg-sunken",
        )}
        {...rest}
      >
        <span
          aria-hidden="true"
          className={cn(
            "size-4.5 rounded-full transition-transform duration-(--dur-quick) ease-out",
            checked ? "translate-x-[1.375rem] bg-charcoal-950" : "translate-x-[0.1875rem] bg-fg-muted",
          )}
        />
      </button>
      <label htmlFor={switchId} className="text-[0.9375rem] text-fg">
        {label}
      </label>
    </div>
  );
}
