export type PageItem = number | "gap";

/**
 * Page numbers to show: always the first and last page, the current page and
 * `siblings` either side, with "gap" where pages are skipped. A gap never
 * hides a single page (that page is shown instead).
 */
export function pageItems(current: number, total: number, siblings = 1): PageItem[] {
  if (total <= 0) return [];
  const page = Math.min(Math.max(current, 1), total);
  const from = Math.max(2, page - siblings);
  const to = Math.min(total - 1, page + siblings);
  const items: PageItem[] = [1];
  if (from === 3) items.push(2);
  else if (from > 3) items.push("gap");
  for (let p = from; p <= to; p++) items.push(p);
  if (to === total - 2) items.push(total - 1);
  else if (to < total - 2) items.push("gap");
  if (total > 1) items.push(total);
  return items;
}
