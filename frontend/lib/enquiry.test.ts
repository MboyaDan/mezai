import { describe, expect, it, vi } from "vitest";
import { emptyEnquiry, submitEnquiry, validateEnquiry, type EnquiryValues } from "./enquiry";

const good: EnquiryValues = {
  ...emptyEnquiry,
  name: "Sample Buyer",
  organisation: "Sample Lab (Sample)",
  email: "buyer@sample.invalid",
  project_type: "ai-evaluation",
};

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

describe("validateEnquiry", () => {
  it("accepts the required fields alone", () => {
    expect(validateEnquiry(good)).toEqual({});
  });

  it("flags each rule the API enforces", () => {
    const errs = validateEnquiry({
      ...good,
      name: "  ",
      organisation: "a\u0007b",
      email: "Sample <buyer@sample.invalid>",
      project_type: "mining",
      volume: "lots",
      message: "x".repeat(5001),
    });
    expect(Object.keys(errs).sort()).toEqual([
      "email",
      "message",
      "name",
      "organisation",
      "project_type",
      "volume",
    ]);
    expect(errs.name).toBe("Enter your name.");
    expect(errs.project_type).toBe("Choose a project type.");
    expect(errs.message).toMatch(/5000/);
  });
});

describe("submitEnquiry", () => {
  it("reports success on 202", async () => {
    const f = vi.fn().mockResolvedValue(json(202, { status: "received" }));
    await expect(submitEnquiry(good, f)).resolves.toEqual({ ok: true });
    expect(f).toHaveBeenCalledWith("/api/v1/enquiries", expect.objectContaining({ method: "POST" }));
  });

  it("maps API field codes to sentences", async () => {
    const f = vi
      .fn()
      .mockResolvedValue(json(422, { error: "invalid_enquiry", fields: { email: "invalid" } }));
    const r = await submitEnquiry(good, f);
    expect(r).toEqual({
      ok: false,
      kind: "fields",
      errors: { email: "Enter an email address like name@example.com." },
    });
  });

  it("retries once with a CSRF token for a signed-in visitor", async () => {
    const f = vi
      .fn()
      .mockResolvedValueOnce(json(403, { error: "csrf_failed" }))
      .mockResolvedValueOnce(json(200, { csrf_token: "tok" }))
      .mockResolvedValueOnce(json(202, { status: "received" }));
    await expect(submitEnquiry(good, f)).resolves.toEqual({ ok: true });
    expect(f.mock.calls[2][1].headers["X-CSRF-Token"]).toBe("tok");
  });

  it("separates rate limits, challenge failures and outages", async () => {
    await expect(submitEnquiry(good, vi.fn().mockResolvedValue(json(429, {})))).resolves.toMatchObject({
      kind: "limited",
    });
    await expect(
      submitEnquiry(good, vi.fn().mockResolvedValue(json(422, { error: "challenge_failed" }))),
    ).resolves.toMatchObject({ kind: "challenge" });
    await expect(submitEnquiry(good, vi.fn().mockRejectedValue(new Error("offline")))).resolves.toMatchObject(
      {
        kind: "failed",
      },
    );
  });
});
