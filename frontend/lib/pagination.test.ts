import { describe, expect, it } from "vitest";
import { pageItems } from "./pagination";

describe("pageItems", () => {
  it("handles empty and single pages", () => {
    expect(pageItems(1, 0)).toEqual([]);
    expect(pageItems(1, 1)).toEqual([1]);
  });

  it("shows every page when there are few", () => {
    expect(pageItems(3, 5)).toEqual([1, 2, 3, 4, 5]);
  });

  it("adds gaps around the current page", () => {
    expect(pageItems(10, 20)).toEqual([1, "gap", 9, 10, 11, "gap", 20]);
  });

  it("never hides a single page behind a gap", () => {
    expect(pageItems(4, 20)).toEqual([1, 2, 3, 4, 5, "gap", 20]);
    expect(pageItems(17, 20)).toEqual([1, "gap", 16, 17, 18, 19, 20]);
  });

  it("clamps out-of-range pages", () => {
    expect(pageItems(99, 3)).toEqual([1, 2, 3]);
    expect(pageItems(-1, 3)).toEqual([1, 2, 3]);
  });
});
