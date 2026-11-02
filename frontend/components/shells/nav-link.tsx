"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

/** Active when the path equals href, or is below it (except for section roots like "/c"). */
export function useIsActive(href: string, exact = false) {
  const path = usePathname();
  return exact ? path === href : path === href || path.startsWith(`${href}/`);
}

export function NavLink({
  href,
  exact,
  className,
  onClick,
  children,
}: {
  href: string;
  exact?: boolean;
  onClick?: () => void;
  className: (active: boolean) => string;
  children: ReactNode;
}) {
  const active = useIsActive(href, exact);
  return (
    <Link
      href={href}
      onClick={onClick}
      aria-current={active ? "page" : undefined}
      className={className(active)}
    >
      {children}
    </Link>
  );
}
