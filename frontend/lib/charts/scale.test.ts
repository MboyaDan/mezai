import { describe, expect, it } from "vitest";
import { linearScale, niceStep, niceTicks } from "./scale";

describe("linearScale", () => {
  it("maps domain to range, including inverted ranges", () => {
    const y = linearScale([0, 100], [200, 0]);
    expect(y(0)).toBe(200);
    expect(y(50)).toBe(100);
    expect(y(100)).toBe(0);
  });

  it("does not divide by zero on a flat domain", () => {
    expect(linearScale([5, 5], [0, 10])(5)).toBe(0);
  });
});

describe("niceTicks", () => {
  it("picks 1, 2 or 5 steps", () => {
    expect(niceStep(100, 4)).toBe(50);
    expect(niceStep(1, 4)).toBe(0.5);
    expect(niceStep(0.7, 4)).toBe(0.2);
  });

  it("covers the range with round values", () => {
    expect(niceTicks(0, 1, 4)).toEqual([0, 0.5, 1]);
    expect(niceTicks(0, 0.7, 4)).toEqual([0, 0.2, 0.4, 0.6, 0.8]);
    expect(niceTicks(3, 97, 4)).toEqual([0, 50, 100]);
    expect(niceTicks(0, 1240, 4)).toEqual([0, 500, 1000, 1500]);
  });

  it("returns one tick for a flat series", () => {
    expect(niceTicks(2, 2)).toEqual([2]);
  });
});
