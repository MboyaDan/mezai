import Link from "next/link";
import type { ComponentProps, ReactNode } from "react";
import { ArrowRight, LoaderCircle } from "lucide-react";
import { cn } from "@/lib/cn";

type Variant = "primary" | "secondary" | "ghost" | "inverse" | "danger";
type Size = "sm" | "md" | "lg";

type StyleProps = { variant?: Variant; size?: Size; fullWidth?: boolean };

const base =
  "group inline-flex items-center justify-center gap-2.5 rounded-control border font-medium whitespace-nowrap select-none " +
  "transition-[background-color,border-color,color,transform] duration-(--dur-quick) ease-out " +
  "active:scale-[0.98] active:duration-(--dur-instant) disabled:pointer-events-none disabled:opacity-50 " +
  "aria-disabled:pointer-events-none aria-disabled:opacity-50";

// Primary is lime with charcoal text on every surface; the charcoal border keeps it visible on ivory.
const variants: Record<Variant, string> = {
  primary: "border-charcoal-950 bg-lime text-charcoal-950 hover:bg-lime-hover",
  secondary: "border-border bg-transparent text-fg hover:border-border-strong",
  ghost: "border-transparent bg-transparent text-fg hover:bg-sunken",
  inverse: "border-inverse-bg bg-inverse-bg text-inverse-fg hover:opacity-90",
  danger: "border-danger bg-danger text-white hover:opacity-90",
};

const sizes: Record<Size, string> = {
  sm: "min-h-9 px-3 text-sm",
  md: "min-h-11 px-4 text-[0.9375rem]",
  lg: "min-h-12 px-5 text-base",
};

export function buttonClasses({ variant = "primary", size = "md", fullWidth }: StyleProps = {}) {
  return cn(base, variants[variant], sizes[size], fullWidth && "w-full");
}

function Arrow() {
  return (
    <ArrowRight
      aria-hidden="true"
      className="size-4 transition-transform duration-(--dur-quick) ease-out group-hover:translate-x-1"
    />
  );
}

type ButtonProps = ComponentProps<"button"> &
  StyleProps & { arrow?: boolean; loading?: boolean; icon?: ReactNode };

export function Button({
  variant,
  size,
  fullWidth,
  arrow,
  loading,
  icon,
  className,
  children,
  disabled,
  type = "button",
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      className={cn(buttonClasses({ variant, size, fullWidth }), className)}
      {...rest}
    >
      {loading ? <LoaderCircle aria-hidden="true" className="size-4 animate-spin" /> : icon}
      {children}
      {arrow && !loading && <Arrow />}
    </button>
  );
}

type ButtonLinkProps = ComponentProps<typeof Link> & StyleProps & { arrow?: boolean };

export function ButtonLink({
  variant,
  size,
  fullWidth,
  arrow,
  className,
  children,
  ...rest
}: ButtonLinkProps) {
  return (
    <Link className={cn(buttonClasses({ variant, size, fullWidth }), className)} {...rest}>
      {children}
      {arrow && <Arrow />}
    </Link>
  );
}

type IconButtonProps = Omit<ComponentProps<"button">, "children"> &
  Omit<StyleProps, "fullWidth"> & { label: string; icon: ReactNode };

/** Square button with an icon only. `label` is required and becomes the accessible name. */
export function IconButton({
  label,
  icon,
  variant = "ghost",
  size = "md",
  className,
  ...rest
}: IconButtonProps) {
  const square = { sm: "size-9 px-0", md: "size-11 px-0", lg: "size-12 px-0" }[size];
  return (
    <button
      type="button"
      aria-label={label}
      className={cn(buttonClasses({ variant, size }), square, className)}
      {...rest}
    >
      <span aria-hidden="true" className="[&>svg]:size-[1.125rem]">
        {icon}
      </span>
    </button>
  );
}
