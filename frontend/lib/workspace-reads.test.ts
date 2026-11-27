import { describe, expect, it } from "vitest";
import {
  ATTEMPTS_PATH,
  CONTRIBUTORS_PATH,
  contributorsTab,
  contributorsTabPath,
  queuePath,
} from "./workspace-reads";

describe("contributorsTab", () => {
  it("opens on everyone for people who cannot approve, whatever the link says", () => {
    expect(contributorsTab(false, "queue")).toBe("all");
    expect(contributorsTab(false, "identity")).toBe("all");
    expect(contributorsTab(false, null)).toBe("all");
  });
  it("opens approvers on the named tab, or the queue", () => {
    expect(contributorsTab(true, "identity")).toBe("identity");
    expect(contributorsTab(true, "erasure")).toBe("erasure");
    expect(contributorsTab(true, "nonsense")).toBe("queue");
    expect(contributorsTab(true, undefined)).toBe("queue");
  });
});

describe("contributorsTabPath", () => {
  it("reads the queue and the full list, nothing else", () => {
    expect(contributorsTabPath("queue")).toBe(ATTEMPTS_PATH);
    expect(contributorsTabPath("all")).toBe(CONTRIBUTORS_PATH);
    expect(contributorsTabPath("identity")).toBeNull();
  });
});

describe("queuePath", () => {
  it("adds only the filters that are set", () => {
    expect(queuePath("", "")).toBe("/v1/review/items");
    expect(queuePath("p1", "")).toBe("/v1/review/items?project_id=p1");
    expect(queuePath("p1", "low_agreement")).toBe("/v1/review/items?project_id=p1&reason=low_agreement");
  });
});
