import { describe, expect, it } from "vitest";

import { apiRequestHeaders, CLIENT_IP_HEADER, PROXY_TOKEN_HEADER } from "./client-ip";

const secret = "s".repeat(64);

describe("apiRequestHeaders", () => {
  it("adds the visitor address and secret from Vercel's x-real-ip", () => {
    const h = apiRequestHeaders(
      new Headers({ "x-real-ip": "197.248.10.20", "x-forwarded-for": "1.1.1.1" }),
      secret,
    );
    expect(h.get(CLIENT_IP_HEADER)).toBe("197.248.10.20");
    expect(h.get(PROXY_TOKEN_HEADER)).toBe(secret);
  });

  it("falls back to the first x-forwarded-for address", () => {
    const h = apiRequestHeaders(new Headers({ "x-forwarded-for": "197.248.10.20, 10.0.0.1" }), secret);
    expect(h.get(CLIENT_IP_HEADER)).toBe("197.248.10.20");
  });

  it("replaces copies a visitor sent", () => {
    const h = apiRequestHeaders(
      new Headers({
        [CLIENT_IP_HEADER]: "6.6.6.6",
        [PROXY_TOKEN_HEADER]: "guess",
        "x-real-ip": "197.248.10.20",
      }),
      secret,
    );
    expect(h.get(CLIENT_IP_HEADER)).toBe("197.248.10.20");
    expect(h.get(PROXY_TOKEN_HEADER)).toBe(secret);
  });

  it("without a secret, sends neither header and drops a visitor's copies", () => {
    const h = apiRequestHeaders(
      new Headers({ [CLIENT_IP_HEADER]: "6.6.6.6", [PROXY_TOKEN_HEADER]: "guess", "x-real-ip": "1.2.3.4" }),
      undefined,
    );
    expect(h.has(CLIENT_IP_HEADER)).toBe(false);
    expect(h.has(PROXY_TOKEN_HEADER)).toBe(false);
  });

  it("without any address, sends no secret", () => {
    const h = apiRequestHeaders(new Headers(), secret);
    expect(h.has(PROXY_TOKEN_HEADER)).toBe(false);
  });
});
