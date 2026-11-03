import { describe, expect, it } from "vitest";
import { sortRows } from "./sort";

describe("sortRows", () => {
  const rows = [
    { id: "item-10", score: 0.5 },
    { id: "item-2", score: 0.9 },
    { id: "item-1", score: null },
    { id: "item-3", score: 0.5 },
  ];

  it("sorts strings naturally", () => {
    expect(sortRows(rows, (r) => r.id, "asc").map((r) => r.id)).toEqual([
      "item-1",
      "item-2",
      "item-3",
      "item-10",
    ]);
  });

  it("sorts numbers both ways and keeps blanks last", () => {
    expect(sortRows(rows, (r) => r.score, "asc").map((r) => r.id)).toEqual([
      "item-10",
      "item-3",
      "item-2",
      "item-1",
    ]);
    expect(sortRows(rows, (r) => r.score, "desc").map((r) => r.id)).toEqual([
      "item-2",
      "item-10",
      "item-3",
      "item-1",
    ]);
  });

  it("is stable and does not mutate the input", () => {
    const copy = [...rows];
    sortRows(rows, (r) => r.score, "desc");
    expect(rows).toEqual(copy);
  });

  it("sorts dates", () => {
    const d = [{ at: new Date("2026-02-01") }, { at: new Date("2026-01-01") }];
    expect(sortRows(d, (r) => r.at, "asc")[0].at.getMonth()).toBe(0);
  });
});
