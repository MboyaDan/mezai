"use client";

import { useEffect, useId, useRef, useState } from "react";
import { LogOut } from "lucide-react";
import { IconButton } from "@/components/ui/button";
import { useSession } from "@/lib/session";

/** Initials button with a one-item menu (sign out). Escape and outside clicks close it. */
export function AccountMenu({ initials, name, email }: { initials: string; name: string; email: string }) {
  const [open, setOpen] = useState(false);
  const menuId = useId();
  const ref = useRef<HTMLDivElement>(null);
  const button = useRef<HTMLButtonElement>(null);
  const { signOut } = useSession();

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        button.current?.focus();
      }
    };
    const onClick = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClick);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClick);
    };
  }, [open]);

  const out = () => signOutAndLeave(signOut);

  return (
    <div ref={ref} className="relative ml-auto md:ml-0">
      <button
        ref={button}
        type="button"
        aria-expanded={open}
        aria-controls={menuId}
        onClick={() => setOpen((o) => !o)}
        className="grid size-9 place-items-center rounded-full border border-border bg-surface-raised font-mono text-xs font-semibold text-fg"
      >
        <span className="sr-only">Account menu for </span>
        {initials}
      </button>
      {open && (
        <div
          id={menuId}
          className="absolute top-11 right-0 z-40 w-64 rounded-card border border-border bg-surface p-3 shadow-float"
        >
          <p className="truncate text-sm font-medium text-fg">{name}</p>
          <p className="truncate text-xs text-fg-muted">{email}</p>
          <button
            type="button"
            onClick={out}
            className="mt-3 flex w-full items-center gap-2 rounded-control px-2 py-2 text-left text-sm text-fg hover:bg-sunken"
          >
            <LogOut aria-hidden="true" className="size-4" />
            Sign out
          </button>
        </div>
      )}
    </div>
  );
}

/** Compact sign-out for the workspace sidebar. */
export function SignOutButton() {
  const { signOut } = useSession();
  return <IconButton label="Sign out" icon={<LogOut />} onClick={() => signOutAndLeave(signOut)} />;
}

/**
 * Signs out, then loads /login as a full page. A client-side replace raced
 * the signed-in areas' own redirect to /login?next=… and could leave the
 * browser on the old page; a full load also drops every cached page.
 */
async function signOutAndLeave(signOut: () => Promise<void>) {
  await signOut().catch(() => undefined);
  window.location.replace("/login");
}

export const initialsOf = (name: string) =>
  name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]!.toUpperCase())
    .join("") || "?";
