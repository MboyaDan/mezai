import Link from "next/link";
import type { ComponentProps } from "react";
import { ArrowUpRight } from "lucide-react";
import { cn } from "@/lib/cn";

type TextLinkProps = ComponentProps<typeof Link> & { tone?: "default" | "accent" };

const isExternal = (href: TextLinkProps["href"]) => typeof href === "string" && /^https?:\/\//.test(href);

/** Inline link. External links open in a new tab, say so to screen readers and show an arrow. */
export function TextLink({ href, tone = "default", className, children, ...rest }: TextLinkProps) {
  const external = isExternal(href);
  return (
    <Link
      href={href}
      className={cn(
        "font-medium underline decoration-1 underline-offset-[0.3em] transition-colors duration-(--dur-quick)",
        tone === "accent"
          ? "text-accent-fg decoration-accent-fg/40 hover:decoration-accent-fg"
          : "text-fg decoration-border hover:decoration-accent-fg",
        className,
      )}
      {...(external ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      {...rest}
    >
      {children}
      {external && (
        <>
          <ArrowUpRight aria-hidden="true" className="ml-0.5 inline size-[0.9em] align-[-0.1em]" />
          <span className="sr-only"> (opens in a new tab)</span>
        </>
      )}
    </Link>
  );
}
