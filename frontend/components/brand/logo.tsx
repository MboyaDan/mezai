import { cn } from "@/lib/cn";
import { brand } from "@/lib/brand";

// Paths copied from docs/brand/mezai-logo (mezai-logo-horizontal.svg, mezai-symbol.svg).
// Letterforms use currentColor so the logo follows the surface; the dot is always lime.
const MARK =
  "M0.00 0.00L122.30 0.00L412.16 441.86A125.0 125.0 0 0 0 335.34 499.48L99.00 156.46L99.00 840.00L0.00 840.00ZM886.00 0.00L763.70 0.00L473.84 441.86A125.0 125.0 0 0 1 550.66 499.48L787.00 156.46L787.00 840.00L886.00 840.00Z";
const WORDMARK =
  "M1091.0 595.0V244.99999999999994H1192.5L1269.0 559.0H1277.0L1353.5 244.99999999999994H1455.0V595.0H1402.5V282.99999999999994H1394.5L1318.5 595.0H1227.5L1151.5 282.99999999999994H1143.5V595.0ZM1641.0 602.0Q1603.5 602.0 1575.5 586.25Q1547.5 570.5 1531.75 541.75Q1516.0 513.0 1516.0 475.0V469.0Q1516.0 430.5 1531.5 401.75Q1547.0 372.99999999999994 1574.75 357.24999999999994Q1602.5 341.49999999999994 1639.0 341.49999999999994Q1674.5 341.49999999999994 1701.25 357.24999999999994Q1728.0 372.99999999999994 1743.0 401.0Q1758.0 429.0 1758.0 466.5V486.0H1568.5Q1569.5 518.5 1590.25 537.75Q1611.0 557.0 1642.0 557.0Q1671.0 557.0 1685.75 544.0Q1700.5 531.0 1708.5 514.0L1751.0 536.0Q1744.0 550.0 1730.75 565.5Q1717.5 581.0 1696.0 591.5Q1674.5 602.0 1641.0 602.0ZM1569.0 446.5H1705.5Q1703.5 418.5 1685.5 402.75Q1667.5 386.99999999999994 1638.5 386.99999999999994Q1609.5 386.99999999999994 1591.25 402.75Q1573.0 418.5 1569.0 446.5ZM1802.0 595.0V532.0L1947.5 398.99999999999994V391.99999999999994H1807.0V348.49999999999994H2003.5V412.0L1857.5 545.0V551.5H2007.5V595.0ZM2042.5 595.0 2142.5 244.99999999999994H2237.5L2337.0 595.0H2281.5L2258.5 512.5H2121.5L2098.5 595.0ZM2135.0 462.5H2245.0L2194.0 278.49999999999994H2186.0ZM2383.0 595.0V244.99999999999994H2437.0V595.0Z";

type LogoProps = {
  variant?: "horizontal" | "symbol";
  className?: string;
  /** Accessible name; pass "" when the logo sits inside a link that already names it. */
  title?: string;
};

export function Logo({ variant = "horizontal", className, title = brand.name }: LogoProps) {
  const horizontal = variant === "horizontal";
  return (
    <svg
      viewBox={horizontal ? "0 0 2437 840" : "0 0 886 840"}
      className={cn("h-7 w-auto", className)}
      role={title ? "img" : undefined}
      aria-label={title || undefined}
      aria-hidden={title ? undefined : true}
      focusable="false"
    >
      <path d={MARK} fill="currentColor" />
      <circle cx="443" cy="563" r="76" fill="#C8F031" />
      {horizontal && <path d={WORDMARK} fill="currentColor" />}
    </svg>
  );
}
