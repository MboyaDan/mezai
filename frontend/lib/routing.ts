/**
 * The routing rule shown in the home page calculator, simplified from brief
 * 8.3: a strict majority label becomes the consensus; no majority sends the
 * item to review. (The real engine also routes flags and a random audit
 * sample; Stage 10.)
 */
export type Routing =
  { kind: "consensus"; label: string; agreeing: number; total: number } | { kind: "review"; total: number };

export function routeItem(labels: readonly string[]): Routing {
  const counts = new Map<string, number>();
  for (const l of labels) counts.set(l, (counts.get(l) ?? 0) + 1);
  let best = "";
  let top = 0;
  for (const [l, n] of counts) {
    if (n > top) {
      best = l;
      top = n;
    }
  }
  return top * 2 > labels.length
    ? { kind: "consensus", label: best, agreeing: top, total: labels.length }
    : { kind: "review", total: labels.length };
}
