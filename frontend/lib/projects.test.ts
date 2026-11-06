import { describe, expect, it } from "vitest";
import type { Project } from "@/lib/api/client";
import {
  actionsFor,
  centsToKes,
  emptyForm,
  formatKes,
  formFromProject,
  kesToCents,
  listFromText,
  percentToRate,
  rateToPercent,
  readiness,
  toInput,
  toKey,
} from "./projects";

describe("money", () => {
  it("parses KES to integer cents without floats", () => {
    expect(kesToCents("12.5")).toBe(1250);
    expect(kesToCents("12.50")).toBe(1250);
    expect(kesToCents("0.07")).toBe(7);
    expect(kesToCents("1,000")).toBe(100000);
    expect(kesToCents("0.1")).toBe(10);
    // 0.29 * 100 is 28.999... as a float; the parser must not use it.
    expect(kesToCents("0.29")).toBe(29);
  });
  it("refuses bad amounts", () => {
    for (const s of ["", "-1", "1.234", "abc", "1e3", "1234567"]) expect(kesToCents(s)).toBeNull();
  });
  it("formats cents", () => {
    expect(centsToKes(1250)).toBe("12.50");
    expect(centsToKes(7)).toBe("0.07");
    expect(formatKes(123456789)).toBe("KES 1,234,567.89");
  });
});

describe("gold rate", () => {
  it("converts percent to share and back", () => {
    expect(percentToRate("10")).toBe(0.1);
    expect(percentToRate("12.5")).toBe(0.125);
    expect(percentToRate("0")).toBe(0);
    expect(percentToRate("51")).toBeNull();
    expect(percentToRate("x")).toBeNull();
    expect(rateToPercent(0.125)).toBe("12.5");
  });
});

describe("lists and keys", () => {
  it("splits lines and commas, dropping blanks and repeats", () => {
    expect(listFromText("Billing\n network, Billing\n\nbilling\nOther")).toEqual([
      "Billing",
      "network",
      "Other",
    ]);
  });
  it("makes rubric keys", () => {
    expect(toKey("Factual accuracy")).toBe("factual_accuracy");
    expect(toKey("  Safety & tone! ")).toBe("safety_tone");
    expect(toKey("2nd pass")).toBe("nd_pass");
  });
});

describe("lifecycle", () => {
  it("offers only the moves the API allows", () => {
    expect(actionsFor("draft").map((a) => a.action)).toEqual(["ready", "close"]);
    expect(actionsFor("ready").map((a) => a.action)).toEqual(["publish", "draft", "close"]);
    expect(actionsFor("live").map((a) => a.action)).toEqual(["pause", "close"]);
    expect(actionsFor("paused").map((a) => a.action)).toEqual(["resume", "close"]);
    expect(actionsFor("closed")).toEqual([]);
  });
});

const project = (over: Partial<Project> = {}): Project => ({
  id: "p",
  org_id: "o",
  org_name: "Sample Client Ltd (Sample)",
  task_type: "pairwise",
  name: "Sample",
  status: "draft",
  instructions_md: "Read both.",
  config: { criteria: ["helpfulness", "safety"], scale: [1, 5] },
  overlap_k: 3,
  gold_rate: 0.1,
  target_count: 500,
  deadline: "2026-12-31",
  item_count: 0,
  skills: [{ kind: "language", value: "sw", min_proficiency: "fluent" }],
  published_at: null,
  closed_at: null,
  created_at: "2026-11-07T00:00:00Z",
  updated_at: "2026-11-07T00:00:00Z",
  staff: {
    pay_per_task_cents: 1250,
    currency: "KES",
    qa_checks: {
      block_unchanged_translation: false,
      warn_fast_seconds: 5,
      warn_rationale_mismatch: true,
      warn_pasted: true,
    },
    min_rationale_chars: 40,
    gold_count: 0,
    missing: ["items", "gold"],
    config_locked: false,
  },
  ...over,
});

describe("readiness", () => {
  it("lists gold only when the gold rate is above 0", () => {
    expect(readiness(project()).map((s) => [s.key, s.done])).toEqual([
      ["instructions", true],
      ["items", false],
      ["gold", false],
    ]);
    expect(readiness(project({ gold_rate: 0 })).map((s) => s.key)).toEqual(["instructions", "items"]);
  });
});

describe("project form", () => {
  it("round-trips a project into the same API body", () => {
    const { input, errors } = toInput(formFromProject(project()), false);
    expect(errors).toEqual({});
    expect(input).toEqual({
      name: "Sample",
      instructions_md: "Read both.",
      config: { criteria: ["helpfulness", "safety"] },
      qa_checks: {
        block_unchanged_translation: false,
        warn_fast_seconds: 5,
        warn_rationale_mismatch: true,
        warn_pasted: true,
      },
      overlap_k: 3,
      gold_rate: 0.1,
      pay_per_task_cents: 1250,
      target_count: 500,
      deadline: "2026-12-31",
      min_rationale_chars: 40,
      skills: [{ kind: "language", value: "sw", min_proficiency: "fluent" }],
    });
  });

  it("sends organisation and task type only on create", () => {
    const f = { ...emptyForm("classification", "org-1"), name: "X", labels: "A\nB", pay_kes: "5" };
    const { input } = toInput(f, true);
    expect(input?.org_id).toBe("org-1");
    expect(input?.task_type).toBe("classification");
    expect(input?.config).toEqual({ labels: ["A", "B"], multi_label: false });
    expect(input?.min_rationale_chars).toBe(0);
    expect(input?.qa_checks?.warn_rationale_mismatch).toBe(false);
  });

  it("reports every problem at once, keyed like the API", () => {
    const { input, errors } = toInput(
      { ...emptyForm("classification"), overlap_k: "9", gold_percent: "70" },
      true,
    );
    expect(input).toBeUndefined();
    expect(Object.keys(errors).sort()).toEqual(
      ["config.labels", "gold_rate", "name", "org_id", "overlap_k", "pay_per_task_cents"].sort(),
    );
  });

  it("drops checks that do not apply to the task type", () => {
    const f = { ...emptyForm("classification", "o"), name: "X", labels: "A\nB", pay_kes: "1" };
    f.qa = { ...f.qa, block_unchanged_translation: true, warn_rationale_mismatch: true };
    const { input } = toInput(f, true);
    expect(input?.qa_checks?.block_unchanged_translation).toBe(false);
    expect(input?.qa_checks?.warn_rationale_mismatch).toBe(false);
  });
});
