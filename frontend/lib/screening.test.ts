import { describe, expect, it } from "vitest";
import { interviewComplete, interviewPasses, type InterviewForm } from "./screening";

const ok: InterviewForm = {
  identity_confirmed: true,
  confidentiality_ok: true,
  scenario_score: 3,
  language_scores: { sw: 4, en: 3 },
};

describe("interviewPasses", () => {
  it("mirrors the API rubric", () => {
    expect(interviewPasses(ok)).toBe(true);
    expect(interviewPasses({ ...ok, identity_confirmed: false })).toBe(false);
    expect(interviewPasses({ ...ok, confidentiality_ok: false })).toBe(false);
    expect(interviewPasses({ ...ok, scenario_score: 2 })).toBe(false);
    expect(interviewPasses({ ...ok, language_scores: { sw: 4, en: 2 } })).toBe(false);
    expect(interviewPasses({ ...ok, language_scores: {} })).toBe(false);
  });
  it("knows when every score is in", () => {
    expect(interviewComplete(ok)).toBe(true);
    expect(interviewComplete({ ...ok, scenario_score: null })).toBe(false);
    expect(interviewComplete({ ...ok, language_scores: { sw: 4, en: null } })).toBe(false);
  });
});
