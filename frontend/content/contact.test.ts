import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { projectTypes, timelines, volumes } from "./contact";

// The API stores these keys; a mismatch would make a valid choice fail.
const goSource = readFileSync(join(__dirname, "../../backend/internal/httpapi/enquiries.go"), "utf8");

function goKeys(mapName: string): string[] {
  const block = goSource.match(new RegExp(`${mapName} = map\\[string\\]string\\{([\\s\\S]*?)\\n\\t\\}`));
  if (!block) throw new Error(`${mapName} not found`);
  return [...block[1].matchAll(/"([^"]*)":/g)].map((m) => m[1]).filter(Boolean);
}

describe("enquiry choices match the API", () => {
  it.each([
    ["enquiryProjectTypes", projectTypes],
    ["enquiryVolumes", volumes],
    ["enquiryTimelines", timelines],
  ])("%s", (name, options) => {
    expect(options.map((o) => o.value).sort()).toEqual(goKeys(name).sort());
  });
});
