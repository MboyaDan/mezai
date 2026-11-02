// Every brand string lives here so a rename (DG-01) is a one-file change.
// Fields without confirmed content stay empty and the UI hides them.
export const brand = {
  name: "MezAI",
  descriptor: "a division of ELS Software Solutions",
  legalEntity: "ELS Software Solutions",
  city: "Nairobi",
  domain: "", // Q5: default domains until DG-01
  contactEmail: "", // Q2: comes from docs/content/ when provided
} as const;

export type Brand = typeof brand;
