/* Motion tokens in seconds, mirroring the CSS variables in app/globals.css
   (DESIGN.md section 9). Keep the two in step. */
export const dur = {
  instant: 0.1,
  quick: 0.18,
  base: 0.28,
  slow: 0.6,
} as const;

export const ease = {
  out: [0.16, 1, 0.3, 1],
  inOut: [0.65, 0, 0.35, 1],
  in: [0.7, 0, 0.84, 0],
} as const;

/** Stagger between group items; DESIGN.md caps a staggered group at 8 items. */
export const staggerStep = 0.06;
export const staggerMax = 8;
