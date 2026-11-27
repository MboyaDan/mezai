import { describe, expect, it } from "vitest";
import { googleError, googleStartUrl } from "./google";

describe("googleStartUrl", () => {
  it("sends only the intent for sign-in", () => {
    expect(googleStartUrl("signin", { privacy_notice: "0.5" })).toBe(
      "/api/v1/auth/google/start?intent=signin",
    );
  });
  it("carries the accepted versions for sign-up", () => {
    expect(googleStartUrl("signup", { privacy_notice: "0.5", contributor_terms: "0.4" })).toBe(
      "/api/v1/auth/google/start?intent=signup&privacy_notice=0.5&contributor_terms=0.4",
    );
  });
});

describe("googleError", () => {
  it("explains every code the API sends", () => {
    for (const code of [
      "google_off",
      "google_failed",
      "google_expired",
      "google_cancelled",
      "google_no_account",
      "google_use_password",
      "google_unverified",
      "consent_required",
    ])
      expect(googleError(code), code).toBeTruthy();
  });
  it("ignores unknown and inherited codes, so the address bar cannot inject text", () => {
    expect(googleError("<b>hi</b>")).toBeNull();
    expect(googleError("toString")).toBeNull();
    expect(googleError(null)).toBeNull();
  });
});
