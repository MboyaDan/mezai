"use client";

import { useMemo, useState, type ReactNode } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { cn } from "@/lib/cn";
import { sortRows, type SortDir, type SortValue } from "@/lib/sort";

export type Column<T> = {
  key: string;
  header: ReactNode;
  cell: (row: T) => ReactNode;
  /** Makes the column sortable by this value. */
  sortValue?: (row: T) => SortValue;
  /** Numbers, scores and money: right-aligned, mono, tabular. */
  numeric?: boolean;
  className?: string;
};

/**
 * Table: client-side sort for the small row counts of the pilot. Server
 * pages pass rows in; paging lives outside (Pagination). Motion is limited to
 * row highlight (DESIGN.md section 9).
 */
export function Table<T>({
  caption,
  columns,
  rows,
  rowKey,
  empty,
  initialSort,
  hideCaption,
  className,
}: {
  caption: string;
  columns: Column<T>[];
  rows: readonly T[];
  rowKey: (row: T) => string;
  /** Shown in place of rows when there are none: an EmptyState or a sentence. */
  empty: ReactNode;
  initialSort?: { key: string; dir: SortDir };
  hideCaption?: boolean;
  className?: string;
}) {
  const [sort, setSort] = useState(initialSort);
  const sorted = useMemo(() => {
    const col = sort && columns.find((c) => c.key === sort.key);
    return col?.sortValue && sort ? sortRows(rows, col.sortValue, sort.dir) : rows;
  }, [rows, columns, sort]);

  const toggle = (key: string) =>
    setSort((s) => (s?.key === key ? { key, dir: s.dir === "asc" ? "desc" : "asc" } : { key, dir: "asc" }));

  return (
    <div className={cn("min-w-0 rounded-card border border-border bg-surface", className)}>
      {/* Focusable region so keyboard users can scroll wide tables at 360 px. */}
      <div role="region" aria-label={caption} tabIndex={0} className="overflow-x-auto">
        <table className="w-full border-collapse text-sm">
          <caption
            className={cn(
              "px-4 pt-4 pb-2 text-left font-display text-base font-semibold",
              hideCaption && "sr-only",
            )}
          >
            {caption}
          </caption>
          <thead>
            <tr className="border-b border-border">
              {columns.map((col) => {
                const active = sort?.key === col.key;
                const Icon = !active ? ArrowUpDown : sort.dir === "asc" ? ArrowUp : ArrowDown;
                return (
                  <th
                    key={col.key}
                    scope="col"
                    aria-sort={active ? (sort.dir === "asc" ? "ascending" : "descending") : undefined}
                    className={cn(
                      "px-4 py-2.5 text-xs font-semibold tracking-wide whitespace-nowrap text-fg-muted uppercase",
                      col.numeric ? "text-right" : "text-left",
                      col.className,
                    )}
                  >
                    {col.sortValue ? (
                      <button
                        type="button"
                        onClick={() => toggle(col.key)}
                        className={cn(
                          "-mx-1 inline-flex items-center gap-1 rounded-control px-1 uppercase hover:text-fg",
                          active && "text-fg",
                          col.numeric && "flex-row-reverse",
                        )}
                      >
                        {col.header}
                        <Icon aria-hidden="true" className={cn("size-3.5", !active && "opacity-50")} />
                      </button>
                    ) : (
                      col.header
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {sorted.map((row) => (
              <tr
                key={rowKey(row)}
                className="border-b border-border transition-colors duration-(--dur-quick) last:border-b-0 hover:bg-sunken"
              >
                {columns.map((col) => (
                  <td
                    key={col.key}
                    className={cn(
                      "px-4 py-3 align-top text-fg",
                      col.numeric && "text-right font-mono tabular",
                      col.className,
                    )}
                  >
                    {col.cell(row)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {/* Outside the scroll region so it fits the viewport, not the table width. */}
      {sorted.length === 0 && <div className="border-t border-border p-4">{empty}</div>}
    </div>
  );
}
