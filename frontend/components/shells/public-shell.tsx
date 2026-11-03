"use client";

import Link from "next/link";
import { useState, type ReactNode } from "react";
import { Menu } from "lucide-react";
import { Logo } from "@/components/brand/logo";
import { ButtonLink, IconButton } from "@/components/ui/button";
import { Drawer } from "@/components/ui/dialog";
import { brand } from "@/lib/brand";
import { cn } from "@/lib/cn";
import { NavLink } from "./nav-link";

// Header navigation from docs/design/sitemap.md. Case studies stay hidden until one is published.
const nav = [
  { href: "/services", label: "Services" },
  { href: "/quality", label: "Quality" },
  { href: "/contributors", label: "Contributors" },
  { href: "/about", label: "About" },
];

const footer = [
  {
    heading: "Work with us",
    links: [
      ...nav.slice(0, 2),
      { href: "/industries", label: "Industries" },
      { href: "/contact", label: "Start a pilot" },
    ],
  },
  {
    heading: "Contributors",
    links: [
      { href: "/contributors", label: "How it works" },
      { href: "/join", label: "Join" },
      { href: "/login", label: "Sign in" },
    ],
  },
  {
    heading: "Legal",
    links: [
      { href: "/legal/privacy", label: "Privacy notice" },
      { href: "/legal/terms", label: "Terms of use" },
      { href: "/legal/contributor-terms", label: "Contributor terms" },
      { href: "/legal/cookies", label: "Cookies" },
    ],
  },
];

const desktopLink = (active: boolean) =>
  cn(
    "text-[0.9375rem] transition-colors duration-(--dur-quick) hover:text-fg",
    active ? "text-fg underline decoration-lime underline-offset-8" : "text-fg-muted",
  );

export function PublicShell({ children }: { children: ReactNode }) {
  const [menuOpen, setMenuOpen] = useState(false);
  return (
    <div className="flex min-h-dvh flex-col">
      <a
        href="#main"
        className="sr-only z-50 bg-lime px-4 py-2 font-medium text-charcoal-950 focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
      >
        Skip to content
      </a>
      <header data-theme="dark" className="border-b border-border bg-bg">
        <div className="mx-auto flex h-16 max-w-[82rem] items-center justify-between gap-6 px-4 sm:px-8 lg:h-[4.5rem] lg:px-16">
          <Link href="/" aria-label={`${brand.name} home`} className="text-fg">
            <Logo title="" className="h-6 lg:h-7" />
          </Link>
          <nav aria-label="Main" className="hidden items-center gap-8 md:flex">
            {nav.map((n) => (
              <NavLink key={n.href} href={n.href} className={desktopLink}>
                {n.label}
              </NavLink>
            ))}
            <ButtonLink href="/contact" variant="secondary" size="sm">
              Talk to us
            </ButtonLink>
          </nav>
          <IconButton
            label="Open menu"
            icon={<Menu />}
            variant="secondary"
            className="md:hidden"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen(true)}
          />
        </div>
      </header>

      <Drawer open={menuOpen} onClose={() => setMenuOpen(false)} title="Menu" className="md:hidden">
        <nav aria-label="Main (mobile)" className="grid">
          {nav.map((n) => (
            <Link
              key={n.href}
              href={n.href}
              onClick={() => setMenuOpen(false)}
              className="border-b border-border py-4 font-display text-2xl"
            >
              {n.label}
            </Link>
          ))}
        </nav>
        <ButtonLink href="/contact" fullWidth arrow className="mt-6" onClick={() => setMenuOpen(false)}>
          Start a pilot project
        </ButtonLink>
      </Drawer>

      <main id="main" className="flex-1">
        {children}
      </main>

      <footer data-theme="dark" className="bg-bg">
        <div className="mx-auto grid max-w-[82rem] gap-12 px-4 py-16 sm:px-8 lg:grid-cols-12 lg:px-16">
          <div className="grid content-start gap-4 lg:col-span-5">
            <Logo title={brand.name} className="h-7 text-fg" />
            <p className="max-w-sm text-sm text-fg-muted">
              {brand.name} is {brand.descriptor}, {brand.city}.
            </p>
          </div>
          {footer.map((col) => (
            <div key={col.heading} className="lg:col-span-2">
              <h2 className="mb-4 font-mono text-xs tracking-wide text-fg-muted uppercase">{col.heading}</h2>
              <ul className="grid gap-2.5 text-sm">
                {col.links.map((l) => (
                  <li key={l.href + l.label}>
                    <Link href={l.href} className="text-fg hover:underline hover:underline-offset-4">
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
        <div className="border-t border-border">
          <p className="mx-auto max-w-[82rem] px-4 py-6 text-xs text-fg-muted sm:px-8 lg:px-16">
            © {brand.legalEntity}. {brand.name} operates under {brand.legalEntity}.
          </p>
        </div>
      </footer>
    </div>
  );
}
