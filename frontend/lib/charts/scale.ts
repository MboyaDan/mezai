/** Maps a value in `domain` linearly onto `range`. */
export function linearScale([d0, d1]: [number, number], [r0, r1]: [number, number]) {
  const span = d1 - d0 || 1;
  return (v: number) => r0 + ((v - d0) / span) * (r1 - r0);
}

/** A "nice" step (1, 2 or 5 times a power of ten) close to span / count. */
export function niceStep(span: number, count: number): number {
  if (span <= 0 || count <= 0) return 1;
  const raw = span / count;
  const power = 10 ** Math.floor(Math.log10(raw));
  const f = raw / power;
  const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10;
  return nice * power;
}

/** About `count` evenly spaced round ticks covering [min, max]. */
export function niceTicks(min: number, max: number, count = 4): number[] {
  if (min === max) return [min];
  const step = niceStep(max - min, count);
  const start = Math.floor(min / step) * step;
  const end = Math.ceil(max / step) * step;
  const ticks: number[] = [];
  // Round to the step's precision so 0.1 + 0.2 does not leak into labels.
  const digits = Math.max(0, -Math.floor(Math.log10(step)));
  for (let v = start; v <= end + step / 2; v += step) ticks.push(Number(v.toFixed(digits)));
  return ticks;
}
