import { describe, expect, it } from "vitest";
import { agreementBand, percentAgreement } from "./agreement";

describe("agreementBand", () => {
  it.each([
    [-0.2, "weak"],
    [0.4, "weak"],
    [0.41, "moderate"],
    [0.6, "moderate"],
    [0.61, "substantial"],
    [0.8, "substantial"],
    [0.81, "strong"],
    [1, "strong"],
  ] as const)("%f is %s", (k, band) => {
    expect(agreementBand(k)).toBe(band);
  });
});

describe("percentAgreement", () => {
  it("is the diagonal share", () => {
    // 2x2 example: 20 + 15 agreed out of 50.
    expect(
      percentAgreement([
        [20, 5],
        [10, 15],
      ]),
    ).toBeCloseTo(0.7, 10);
  });

  it("is null with no items", () => {
    expect(
      percentAgreement([
        [0, 0],
        [0, 0],
      ]),
    ).toBeNull();
  });
});
