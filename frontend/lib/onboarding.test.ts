import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/client";
import {
  consentPayload,
  currentStep,
  errorMessage,
  isComplete,
  maskedPreview,
  normaliseKenyanMobile,
  normaliseNationalId,
  safeNext,
} from "./onboarding";
import { legalDocs } from "@/content/legal";

describe("safeNext", () => {
  it("keeps same-area paths and refuses everything else", () => {
    expect(safeNext("/c/profile", "contributor")).toBe("/c/profile");
    expect(safeNext("/w/contributors", "staff")).toBe("/w/contributors");
    for (const bad of [null, "", "https://evil.example", "//evil.example", "/\\evil", "c/profile"]) {
      expect(safeNext(bad, "contributor")).toBe("/c");
    }
    expect(safeNext("/w", "contributor")).toBe("/c");
    expect(safeNext("/c", "client")).toBe("/w");
  });
});

describe("normaliseKenyanMobile (mirrors the API)", () => {
  it("accepts the usual forms", () => {
    expect(normaliseKenyanMobile("0712 345 678")).toBe("+254712345678");
    expect(normaliseKenyanMobile("0112-345-678")).toBe("+254112345678");
    expect(normaliseKenyanMobile("254712345678")).toBe("+254712345678");
    expect(normaliseKenyanMobile("+254 712 345 678")).toBe("+254712345678");
  });
  it("refuses others", () => {
    for (const bad of ["", "712345678", "0812345678", "+255712345678", "07123456789", "07123x5678"]) {
      expect(normaliseKenyanMobile(bad)).toBeNull();
    }
  });
  it("previews the masked form", () => {
    expect(maskedPreview("0712345678")).toBe("+254 7•• ••• 678");
    expect(maskedPreview("123")).toBeNull();
  });
});

describe("consentPayload", () => {
  it("uses the current version of each document", () => {
    const privacy = legalDocs.find((d) => d.slug === "privacy")!;
    const terms = legalDocs.find((d) => d.slug === "contributor-terms")!;
    expect(consentPayload(["privacy_notice", "contributor_terms"])).toEqual({
      privacy_notice: privacy.version,
      contributor_terms: terms.version,
    });
  });
});

describe("isComplete (mirrors quality.Parse)", () => {
  const cls = { labels: ["A", "B"] };
  const pw = { criteria: ["accuracy", "safety"] };
  const full = { accuracy: 3, safety: 4 };
  it("classification needs one label unless multi-label", () => {
    expect(isComplete("classification", cls, { labels: ["A"] })).toBe(true);
    expect(isComplete("classification", cls, { labels: ["A", "B"] })).toBe(false);
    expect(isComplete("classification", { ...cls, multi_label: true }, { labels: ["A", "B"] })).toBe(true);
    expect(isComplete("classification", cls, { labels: [] })).toBe(false);
    expect(isComplete("classification", cls, undefined)).toBe(false);
  });
  it("pairwise needs a preference, every score for both sides and a rationale", () => {
    const ok = { preference: "a" as const, scores_a: full, scores_b: full, rationale: "A is clearer." };
    expect(isComplete("pairwise", pw, ok)).toBe(true);
    expect(isComplete("pairwise", pw, { ...ok, rationale: "  " })).toBe(false);
    expect(isComplete("pairwise", pw, { ...ok, scores_b: { accuracy: 3 } })).toBe(false);
    expect(isComplete("pairwise", pw, { ...ok, preference: undefined })).toBe(false);
  });
  it("translation review needs adequacy and fluency from 1 to 5", () => {
    expect(isComplete("translation_review", {}, { adequacy: 4, fluency: 5 })).toBe(true);
    expect(isComplete("translation_review", {}, { adequacy: 0, fluency: 5 })).toBe(false);
    expect(isComplete("translation_review", {}, { adequacy: 4 })).toBe(false);
  });
});

describe("normaliseNationalId", () => {
  it("mirrors identity.NormaliseNationalID", () => {
    expect(normaliseNationalId(" 2468 1357 ")).toBe("24681357");
    expect(normaliseNationalId("02468-1357")).toBe("24681357");
    expect(normaliseNationalId("123456")).toBe("123456");
    expect(normaliseNationalId("12345")).toBeNull();
    expect(normaliseNationalId("1234567890")).toBeNull();
    expect(normaliseNationalId("A1234567")).toBeNull();
    expect(normaliseNationalId("000000")).toBeNull();
  });
});

describe("writing sample", () => {
  const words = (n: number) => Array.from({ length: n }, () => "word").join(" ");
  it("needs 60 to 250 words, like the API", () => {
    expect(isComplete("writing", {}, { text: words(59) })).toBe(false);
    expect(isComplete("writing", {}, { text: words(60) })).toBe(true);
    expect(isComplete("writing", {}, { text: words(250) })).toBe(true);
    expect(isComplete("writing", {}, { text: words(251) })).toBe(false);
  });
});

describe("currentStep", () => {
  const p = (
    status: string,
    consent: string[] = [],
    qualifications: unknown[] = [],
    screening = "shortlisted",
    interview = false,
  ) =>
    ({
      status,
      consent_required: consent,
      qualifications,
      screening_status: screening,
      interview_passed: interview,
    }) as Parameters<typeof currentStep>[0];
  it("orders the onboarding steps", () => {
    expect(currentStep(p("applied", ["privacy_notice"]), [])).toBe("consent");
    expect(currentStep(p("applied"), [])).toBe("profile");
    expect(currentStep(p("qualifying"), [])).toBe("qualify");
    expect(currentStep(p("qualifying"), [{ state: "awaiting_review" } as never])).toBe("review");
    // Screening first (D-100), then test, interview, identity.
    expect(currentStep(p("qualifying", [], [], "pending"), [])).toBe("screening");
    expect(currentStep(p("qualifying", [], [], "waitlisted"), [])).toBe("screening");
    expect(currentStep(p("qualifying", [], [], "declined"), [])).toBe("declined");
    // Only the core check counts towards activation; a skill alone does not.
    expect(currentStep(p("qualifying", [], [{ skill_kind: "language" }]), [])).toBe("qualify");
    expect(currentStep(p("qualifying", [], [{ skill_kind: "core" }]), [])).toBe("interview");
    expect(currentStep(p("qualifying", [], [{ skill_kind: "core" }], "shortlisted", true), [])).toBe(
      "identity",
    );
    expect(currentStep(p("active"), [])).toBe("active");
    expect(currentStep(p("paused"), [])).toBe("paused");
    expect(currentStep(p("removed", ["privacy_notice"]), [])).toBe("removed");
  });
});

describe("errorMessage", () => {
  it("shows the API's field message and a sentence for known codes", () => {
    expect(errorMessage(new ApiError(422, "invalid_input", "country", "use a two-letter country code"))).toBe(
      "use a two-letter country code",
    );
    expect(
      errorMessage(new ApiError(422, "weak_password", undefined, "password must be at least 12 characters")),
    ).toBe("Password must be at least 12 characters.");
    expect(errorMessage(new ApiError(0, "network"))).toMatch(/connection/);
    expect(errorMessage(new ApiError(500, "whatever"))).toMatch(/Something went wrong/);
  });
});
