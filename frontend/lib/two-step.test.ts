import { describe, expect, it } from "vitest";
import { groupSecret, recoveryFile, staffOnly } from "./two-step";

describe("two-step helpers", () => {
  it("groups a key in fours", () => {
    expect(groupSecret("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ")).toBe("GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ");
    expect(groupSecret("ABCDEF")).toBe("ABCD EF");
  });
  it("writes every recovery code on its own line with the date", () => {
    const f = recoveryFile(["k7pq-3mzd", "a2bc-defg"], new Date("2026-11-17T08:00:00Z"));
    expect(f).toContain("Made on 2026-11-17.");
    expect(f.split("\n")).toEqual(expect.arrayContaining(["k7pq-3mzd", "a2bc-defg"]));
  });
});

describe("staffOnly", () => {
  it("keeps users with a staff role and drops client users", () => {
    const users = [
      { id: "a", roles: ["ops_manager"] },
      { id: "b", roles: ["client_admin:0192"] },
      { id: "c", roles: ["reviewer", "payout_approver"] },
      { id: "d", roles: [] },
    ];
    expect(staffOnly(users).map((u) => u.id)).toEqual(["a", "c"]);
  });
});
