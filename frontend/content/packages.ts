// Pilot packages with "from" prices (Stage 21a, D-161). Filled only from
// prices ELS has set (docs/content/); while the list is empty the Services
// page shows no packages section. Prices are what a client pays, in KES or
// USD as ELS decides, and are starting points, never quotes.

export type PilotPackage = {
  name: string;
  /** For example "From KES 150,000". */
  from: string;
  summary: string;
  includes: string[];
};

export const pilotPackages: PilotPackage[] = [];
