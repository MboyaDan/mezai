import Link from "next/link";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "@/lib/cn";
import { pageItems } from "@/lib/pagination";

/* Pagination: links, so pages are shareable URLs and work without JavaScript. */
export function Pagination({
  page,
  pageCount,
  hrefFor,
  label = "Pagination",
  className,
}: {
  page: number;
  pageCount: number;
  hrefFor: (page: number) => string;
  label?: string;
  className?: string;
}) {
  if (pageCount <= 1) return null;
  const item =
    "inline-flex min-h-10 min-w-10 items-center justify-center rounded-control border text-sm transition-colors duration-(--dur-quick)";
  const idle = "border-transparent text-fg hover:border-border";
  const off = "pointer-events-none border-transparent text-fg-muted opacity-50";
  const prev = page > 1;
  const next = page < pageCount;

  return (
    <nav aria-label={label} className={className}>
      <ul className="flex flex-wrap items-center gap-1">
        <li>
          {prev ? (
            <Link href={hrefFor(page - 1)} rel="prev" className={cn(item, idle, "gap-1 px-2.5")}>
              <ChevronLeft aria-hidden="true" className="size-4" />
              <span className="max-sm:sr-only">Previous</span>
            </Link>
          ) : (
            <span aria-disabled="true" className={cn(item, off, "gap-1 px-2.5")}>
              <ChevronLeft aria-hidden="true" className="size-4" />
              <span className="max-sm:sr-only">Previous</span>
            </span>
          )}
        </li>
        {pageItems(page, pageCount).map((p, i) =>
          p === "gap" ? (
            <li key={`gap-${i}`} aria-hidden="true" className="px-1 text-fg-muted max-sm:hidden">
              …
            </li>
          ) : (
            <li key={p} className="max-sm:hidden">
              <Link
                href={hrefFor(p)}
                aria-label={`Page ${p}`}
                aria-current={p === page ? "page" : undefined}
                className={cn(
                  item,
                  "font-mono tabular",
                  p === page ? "border-border-strong font-semibold text-fg" : idle,
                )}
              >
                {p}
              </Link>
            </li>
          ),
        )}
        <li aria-current="page" className="px-2 font-mono text-sm text-fg tabular sm:hidden">
          {page} / {pageCount}
        </li>
        <li>
          {next ? (
            <Link href={hrefFor(page + 1)} rel="next" className={cn(item, idle, "gap-1 px-2.5")}>
              <span className="max-sm:sr-only">Next</span>
              <ChevronRight aria-hidden="true" className="size-4" />
            </Link>
          ) : (
            <span aria-disabled="true" className={cn(item, off, "gap-1 px-2.5")}>
              <span className="max-sm:sr-only">Next</span>
              <ChevronRight aria-hidden="true" className="size-4" />
            </span>
          )}
        </li>
      </ul>
    </nav>
  );
}
