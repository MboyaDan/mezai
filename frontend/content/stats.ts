import type { PublicStat } from "@/lib/stats";

// Bundled copy of the published stats, used when the API is asleep or
// unreachable. Keep it equal to what the API publishes. Empty until a real,
// sourced record exists (honesty rule 2: no record, no number).
export const fallbackStats: PublicStat[] = [];
