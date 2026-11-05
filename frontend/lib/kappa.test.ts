import { describe, expect, it } from "vitest";
import { cohensKappa, twoRaterExample } from "./kappa";

describe("cohensKappa", () => {
  it("matches the textbook worked example (Wikipedia, Cohen's kappa: 20, 5, 10, 15)", () => {
    // 50 items: both yes 20, A yes B no 5, A no B yes 10, both no 15.
    const r = cohensKappa([
      [20, 5],
      [10, 15],
    ])!;
    expect(r.po).toBeCloseTo(0.7, 10);
    expect(r.pe).toBeCloseTo(0.5, 10);
    expect(r.kappa).toBeCloseTo(0.4, 10);
  });

  it("matches the second textbook example (45, 15, 25, 15: kappa 0.1304)", () => {
    const r = cohensKappa([
      [45, 15],
      [25, 15],
    ])!;
    expect(r.po).toBeCloseTo(0.6, 10);
    expect(r.pe).toBeCloseTo(0.54, 10);
    expect(r.kappa).toBeCloseTo(0.1304, 4);
  });

  it("is 1 for perfect agreement and can go below 0", () => {
    expect(
      cohensKappa([
        [10, 0],
        [0, 10],
      ])!.kappa,
    ).toBe(1);
    expect(
      cohensKappa([
        [0, 10],
        [10, 0],
      ])!.kappa,
    ).toBe(-1);
  });

  it("is undefined with no items, a non-square matrix, or a single shared label", () => {
    expect(
      cohensKappa([
        [0, 0],
        [0, 0],
      ]),
    ).toBeNull();
    expect(cohensKappa([[1, 2]])).toBeNull();
    expect(
      cohensKappa([
        [5, 0],
        [0, 0],
      ]),
    ).toBeNull();
  });
});

describe("twoRaterExample", () => {
  it("keeps the total and the agreement count", () => {
    const m = twoRaterExample(50, 41, 0.8);
    expect(m.flat().reduce((s, v) => s + v, 0)).toBe(50);
    expect(m[0][0] + m[1][1]).toBe(41);
  });

  it("shows high percent agreement with low kappa when one label dominates", () => {
    const r = cohensKappa(twoRaterExample(50, 45, 0.95))!;
    expect(r.po).toBeCloseTo(0.9, 10);
    expect(r.kappa).toBeLessThan(0.4);
  });
});
