import { describe, expect, it } from "vitest";
import { routeItem } from "./routing";

describe("routeItem", () => {
  it("accepts a unanimous or majority label", () => {
    expect(routeItem(["Billing", "Billing", "Billing"])).toEqual({
      kind: "consensus",
      label: "Billing",
      agreeing: 3,
      total: 3,
    });
    expect(routeItem(["Billing", "Other", "Billing"])).toEqual({
      kind: "consensus",
      label: "Billing",
      agreeing: 2,
      total: 3,
    });
  });

  it("sends a three-way split or a tie to review", () => {
    expect(routeItem(["Billing", "Network", "Other"])).toEqual({ kind: "review", total: 3 });
    expect(routeItem(["Billing", "Network"])).toEqual({ kind: "review", total: 2 });
  });
});
