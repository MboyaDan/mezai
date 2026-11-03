import type { CSSProperties } from "react";
import { cn } from "@/lib/cn";

type SignalLatticeProps = {
  rows?: number;
  cols?: number;
  /** Cell of the lime agreement dot as [row, col]; null for none. */
  accent?: [number, number] | null;
  /**
   * "align-on-load": strokes start at ±25° and rotate into line (hero).
   * "scattered": static, not yet aligned (for scroll stories driven elsewhere).
   * "aligned": static, final state (also what reduced motion shows).
   */
  state?: "align-on-load" | "scattered" | "aligned";
  seed?: number;
  className?: string;
};

const ANGLE = -60;
const SPREAD = 25;

/** Deterministic pseudo-random sequence so server and client render the same angles. */
function angles(n: number, seed: number): number[] {
  let s = seed % 2147483647 || 1;
  return Array.from({ length: n }, () => {
    s = (s * 16807) % 2147483647;
    return ((s / 2147483647) * 2 - 1) * SPREAD;
  });
}

/**
 * The brand motif: short strokes that rotate into alignment, with one lime
 * dot where agreement is reached. Decorative; hidden from assistive tech.
 * It fills its container's width: size it with a wrapper, not a width class.
 */
export function SignalLattice({
  rows = 9,
  cols = 9,
  accent = [Math.floor(rows / 2) + 1, Math.floor(cols / 2)],
  state = "aligned",
  seed = 7,
  className,
}: SignalLatticeProps) {
  const step = 48;
  const len = 18;
  const rad = (ANGLE * Math.PI) / 180;
  const dx = (Math.cos(rad) * len) / 2;
  const dy = (Math.sin(rad) * len) / 2;
  const offsets = angles(rows * cols, seed);

  const cells = [];
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const cx = step * (c + 0.5);
      const cy = step * (r + 0.5);
      const key = `${r}-${c}`;
      if (accent && r === accent[0] && c === accent[1]) {
        cells.push(<circle key={key} cx={cx} cy={cy} r={9} className="lattice-dot fill-lime" />);
        continue;
      }
      const near = accent ? Math.abs(r - accent[0]) + Math.abs(c - accent[1]) <= 2 : false;
      const from = offsets[r * cols + c].toFixed(1);
      cells.push(
        <line
          key={key}
          x1={cx - dx}
          y1={cy - dy}
          x2={cx + dx}
          y2={cy + dy}
          strokeWidth={3}
          strokeLinecap="square"
          className={near ? "stroke-[var(--lattice-stroke-near)]" : "stroke-[var(--lattice-stroke)]"}
          style={{ "--from": `${from}deg` } as CSSProperties}
        />,
      );
    }
  }

  return (
    <svg
      viewBox={`0 0 ${cols * step} ${rows * step}`}
      aria-hidden="true"
      focusable="false"
      data-state={state}
      className={cn("signal-lattice block h-auto w-full", className)}
    >
      {cells}
    </svg>
  );
}
