export type SortDir = "asc" | "desc";
export type SortValue = string | number | Date | null | undefined;

const collator = new Intl.Collator("en", { numeric: true, sensitivity: "base" });

function compare(a: SortValue, b: SortValue): number {
  // Empty values always sort last, whatever the direction.
  if (a == null || a === "") return b == null || b === "" ? 0 : 1;
  if (b == null || b === "") return -1;
  if (typeof a === "string" && typeof b === "string") return collator.compare(a, b);
  return Number(a) - Number(b);
}

/** Returns a sorted copy. Stable: equal values keep their original order. */
export function sortRows<T>(rows: readonly T[], value: (row: T) => SortValue, dir: SortDir): T[] {
  const sign = dir === "asc" ? 1 : -1;
  return rows
    .map((row, i) => ({ row, i, v: value(row) }))
    .sort((x, y) => {
      const blank = (v: SortValue) => v == null || v === "";
      if (blank(x.v) || blank(y.v)) return compare(x.v, y.v) || x.i - y.i;
      return sign * compare(x.v, y.v) || x.i - y.i;
    })
    .map((e) => e.row);
}
