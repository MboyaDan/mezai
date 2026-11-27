import { describe, expect, it } from "vitest";
import { hiddenFromClient, sees, visibilityPanels } from "./visibility";

const staff = { pay_per_task_cents: 0 } as never;
const at = (quality: boolean, workforce: boolean, isStaff = false) => ({
  client_visibility: { quality, workforce },
  staff: isStaff ? staff : undefined,
});

describe("client visibility", () => {
  it("hides a held-back panel from client users only", () => {
    expect(sees(at(false, true), "quality")).toBe(false);
    expect(sees(at(false, true), "workforce")).toBe(true);
    expect(sees(at(false, false, true), "quality")).toBe(true);
  });
  it("tags held-back panels for staff, never for the client", () => {
    expect(hiddenFromClient(at(false, true, true), "quality")).toBe(true);
    expect(hiddenFromClient(at(true, true, true), "quality")).toBe(false);
    expect(hiddenFromClient(at(false, false), "quality")).toBe(false);
  });
  it("names every panel", () => {
    expect(visibilityPanels.map((p) => p.key)).toEqual(["quality", "workforce"]);
  });
});
