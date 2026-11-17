/** A base32 key in groups of four, easier to type into an app. */
export function groupSecret(secret: string): string {
  return (secret.match(/.{1,4}/g) ?? []).join(" ");
}

/** The text file a person keeps their recovery codes in. */
export function recoveryFile(codes: string[], at: Date): string {
  const day = at.toISOString().slice(0, 10);
  return [
    "MezAI recovery codes",
    `Made on ${day}. Each code signs you in once if you lose your phone.`,
    "Keep this file somewhere safe. Turning two-step sign-in off and on again makes new codes.",
    "",
    ...codes,
    "",
  ].join("\n");
}

const staffRoles: Record<string, string> = {
  super_admin: "Super Admin",
  ops_manager: "Ops Manager",
  payout_approver: "Payout Approver",
  reviewer: "Reviewer",
};

/** Staff role keys have no organisation suffix; client roles read "role:org". */
export function staffOnly<T extends { roles: string[] }>(users: T[]): T[] {
  return users.filter((u) => u.roles.some((r) => r in staffRoles));
}

export const staffRoleLabel = (r: string) => staffRoles[r] ?? r;
