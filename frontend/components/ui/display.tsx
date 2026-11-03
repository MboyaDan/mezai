import Link from "next/link";
import type { ComponentProps, ReactNode } from "react";
import {
  ChevronRight,
  CircleAlert,
  CircleCheck,
  CircleDashed,
  Info,
  Plus,
  TriangleAlert,
} from "lucide-react";
import { cn } from "@/lib/cn";
import { SignalLattice } from "@/components/brand/signal-lattice";

/* Card: borders separate, not shadows (DESIGN.md section 8). */
export function Card({ className, interactive, ...rest }: ComponentProps<"div"> & { interactive?: boolean }) {
  return (
    <div
      className={cn(
        "rounded-card border border-border bg-surface",
        interactive && "transition-colors duration-(--dur-quick) hover:border-border-strong/50",
        className,
      )}
      {...rest}
    />
  );
}

export function CardHeader({
  title,
  meta,
  className,
}: {
  title: ReactNode;
  meta?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 px-5 pt-5", className)}>
      <h3 className="font-display text-lg font-semibold">{title}</h3>
      {meta && <span className="text-sm text-fg-muted">{meta}</span>}
    </div>
  );
}

/* Tag: a neutral label such as a skill or language. */
export function Tag({ className, ...rest }: ComponentProps<"span">) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border border-border bg-sunken px-2.5 py-0.5 text-xs text-fg",
        className,
      )}
      {...rest}
    />
  );
}

/* Badge: a small count. */
export function Badge({ className, ...rest }: ComponentProps<"span">) {
  return (
    <span
      className={cn(
        "inline-flex min-w-5 items-center justify-center rounded-full bg-inverse-bg px-1.5 font-mono text-[0.6875rem] leading-5 font-semibold text-inverse-fg tabular",
        className,
      )}
      {...rest}
    />
  );
}

/* StatusPill: status is never colour alone; icon and label always present. */
type Status = "success" | "warning" | "danger" | "neutral" | "info";

const statusStyle: Record<Status, { cls: string; Icon: typeof CircleCheck }> = {
  success: { cls: "border-success text-success", Icon: CircleCheck },
  warning: { cls: "border-warning text-warning", Icon: TriangleAlert },
  danger: { cls: "border-danger text-danger", Icon: CircleAlert },
  info: { cls: "border-border-strong/50 text-fg", Icon: Info },
  neutral: { cls: "border-border text-fg-muted", Icon: CircleDashed },
};

export function StatusPill({
  status,
  children,
  className,
}: {
  status: Status;
  children: ReactNode;
  className?: string;
}) {
  const { cls, Icon } = statusStyle[status];
  return (
    <span
      className={cn(
        // Light surfaces get a paper fill so status text keeps 4.5:1; dark uses ivory-tinted variants below.
        "inline-flex items-center gap-1.5 rounded-full border bg-surface px-2.5 py-0.5 text-[0.8125rem] font-semibold",
        "in-data-[theme=dark]:border-current in-data-[theme=dark]:bg-transparent in-data-[theme=dark]:text-fg",
        cls,
        className,
      )}
    >
      <Icon aria-hidden="true" className="size-3.5" />
      {children}
    </span>
  );
}

/* Alert: inline message. Danger uses role="alert"; others are polite. */
const alertIcon: Record<Status, typeof Info> = {
  success: CircleCheck,
  warning: TriangleAlert,
  danger: CircleAlert,
  info: Info,
  neutral: Info,
};
const alertBar: Record<Status, string> = {
  success: "border-l-success",
  warning: "border-l-warning",
  danger: "border-l-danger",
  info: "border-l-border-strong",
  neutral: "border-l-border",
};
const alertIconColour: Record<Status, string> = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  info: "text-fg",
  neutral: "text-fg-muted",
};

export function Alert({
  tone = "info",
  title,
  children,
  action,
  className,
}: {
  tone?: Status;
  title: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  const Icon = alertIcon[tone];
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      className={cn(
        "flex gap-3 rounded-card border border-l-2 border-border bg-surface p-4",
        alertBar[tone],
        "in-data-[theme=dark]:[&_svg]:text-fg",
        className,
      )}
    >
      <Icon aria-hidden="true" className={cn("mt-0.5 size-5 shrink-0", alertIconColour[tone])} />
      <div className="grid gap-1">
        <p className="font-semibold text-fg">{title}</p>
        {children && <div className="text-sm text-fg-muted">{children}</div>}
        {action && <div className="mt-2">{action}</div>}
      </div>
    </div>
  );
}

/* Skeleton: shaped like the content it replaces. */
export function Skeleton({ className }: { className?: string }) {
  return (
    <div aria-hidden="true" className={cn("animate-skeleton rounded-control bg-border/70", className)} />
  );
}

/* EmptyState: small aligned lattice, one clear action. */
export function EmptyState({
  title,
  children,
  action,
  className,
}: {
  title: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col items-center gap-4 rounded-card border border-dashed border-border px-6 py-12 text-center",
        className,
      )}
    >
      <div className="w-24">
        <SignalLattice rows={3} cols={5} accent={[1, 2]} />
      </div>
      <div className="grid max-w-md gap-1.5">
        <p className="font-display text-lg font-semibold">{title}</p>
        {children && <div className="text-sm text-fg-muted">{children}</div>}
      </div>
      {action}
    </div>
  );
}

/* KeyboardHint: <kbd> keys, e.g. <KeyboardHint keys={["⌘", "Enter"]} />. */
export function KeyboardHint({
  keys,
  label,
  className,
}: {
  keys: string[];
  label?: string;
  className?: string;
}) {
  return (
    <span className={cn("inline-flex items-center gap-1 text-xs text-fg-muted", className)}>
      {keys.map((k) => (
        <kbd
          key={k}
          className="inline-flex min-w-6 items-center justify-center rounded-control border border-b-2 border-border bg-surface px-1.5 font-mono text-[0.6875rem] leading-5 text-fg"
        >
          {k}
        </kbd>
      ))}
      {label && <span className="ml-1">{label}</span>}
    </span>
  );
}

/* Breadcrumb: the last item is the current page. */
export function Breadcrumb({ items }: { items: Array<{ label: string; href?: string }> }) {
  return (
    <nav aria-label="Breadcrumb">
      <ol className="flex flex-wrap items-center gap-1.5 text-sm text-fg-muted">
        {items.map((item, i) => {
          const last = i === items.length - 1;
          return (
            <li key={`${item.label}-${i}`} className="flex items-center gap-1.5">
              {item.href && !last ? (
                <Link href={item.href} className="underline-offset-4 hover:text-fg hover:underline">
                  {item.label}
                </Link>
              ) : (
                <span aria-current={last ? "page" : undefined} className={cn(last && "font-medium text-fg")}>
                  {item.label}
                </span>
              )}
              {!last && <ChevronRight aria-hidden="true" className="size-3.5" />}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}

/* Accordion: native <details>, so it works without JavaScript and with find-in-page. */
export function Accordion({
  items,
  className,
}: {
  items: Array<{ id: string; title: ReactNode; content: ReactNode; open?: boolean }>;
  className?: string;
}) {
  return (
    <div className={cn("border-t border-border", className)}>
      {items.map((item) => (
        <details key={item.id} open={item.open} className="group border-b border-border">
          <summary className="flex min-h-12 cursor-pointer list-none items-center justify-between gap-4 py-3 font-medium [&::-webkit-details-marker]:hidden">
            {item.title}
            <Plus
              aria-hidden="true"
              className="size-4 shrink-0 text-fg-muted transition-transform duration-(--dur-base) group-open:rotate-45"
            />
          </summary>
          <div className="pb-4 text-fg-muted">{item.content}</div>
        </details>
      ))}
    </div>
  );
}
