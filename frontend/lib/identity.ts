// One person, one account (D-078, Stage 17a). The API decides; these helpers
// only choose what the screens say.

/** Mirrors MaxNumberTries and NumberTryWindow in backend/internal/contributors/identity.go (D-137). */
export const MAX_NUMBER_TRIES = 3;
export const NUMBER_TRY_WINDOW_MS = 24 * 60 * 60 * 1000;

type Flag = { raised_by_this: boolean; at: string };

/**
 * When an account has entered several numbers that belong to other accounts,
 * it cannot change the numbers on its profile until the oldest of those
 * tries is a day old. Returns that time, or null when changes are allowed.
 */
export function numberChangesPausedUntil(flags: Flag[], now: Date = new Date()): Date | null {
  const recent = flags
    .filter((f) => f.raised_by_this)
    .map((f) => new Date(f.at).getTime())
    .filter((t) => now.getTime() - t < NUMBER_TRY_WINDOW_MS)
    .sort((a, b) => a - b);
  if (recent.length < MAX_NUMBER_TRIES) return null;
  // Changes reopen when fewer than the maximum remain inside the window.
  return new Date(recent[recent.length - MAX_NUMBER_TRIES] + NUMBER_TRY_WINDOW_MS);
}
