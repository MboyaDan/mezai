import { describe, expect, it } from "vitest";
import { contentSecurityPolicy, securityHeaders } from "./security-headers";

const directive = (csp: string, name: string) =>
  csp
    .split("; ")
    .find((d) => d.startsWith(name + " "))
    ?.slice(name.length + 1);

describe("content security policy", () => {
  const prod = contentSecurityPolicy({ dev: false, turnstile: false });

  it("allows only this origin and can never be framed", () => {
    expect(directive(prod, "default-src")).toBe("'self'");
    expect(directive(prod, "frame-ancestors")).toBe("'none'");
    expect(directive(prod, "object-src")).toBe("'none'");
    expect(directive(prod, "base-uri")).toBe("'self'");
    expect(directive(prod, "form-action")).toBe("'self'");
    expect(directive(prod, "connect-src")).toBe("'self'");
    expect(directive(prod, "frame-src")).toBe("'none'");
  });

  it("names no other origin and no eval in production", () => {
    expect(prod).not.toMatch(/https?:|\*|unsafe-eval|ws:/);
  });

  it("adds Turnstile only when it is switched on", () => {
    const withTurnstile = contentSecurityPolicy({ dev: false, turnstile: true });
    expect(directive(withTurnstile, "script-src")).toContain("https://challenges.cloudflare.com");
    expect(directive(withTurnstile, "frame-src")).toBe("https://challenges.cloudflare.com");
    expect(directive(withTurnstile, "connect-src")).toBe("'self'");
  });

  it("lets next dev hot reload", () => {
    const dev = contentSecurityPolicy({ dev: true, turnstile: false });
    expect(directive(dev, "script-src")).toContain("'unsafe-eval'");
    expect(directive(dev, "connect-src")).toContain("ws:");
  });
});

describe("security headers", () => {
  const keys = (dev: boolean) => securityHeaders({ dev, turnstile: false }).map((h) => h.key);

  it("sends HSTS in production only", () => {
    expect(keys(false)).toContain("Strict-Transport-Security");
    expect(keys(true)).not.toContain("Strict-Transport-Security");
  });

  it("always refuses framing and sniffing", () => {
    for (const dev of [true, false]) {
      const h = Object.fromEntries(securityHeaders({ dev, turnstile: false }).map((x) => [x.key, x.value]));
      expect(h["X-Frame-Options"]).toBe("DENY");
      expect(h["X-Content-Type-Options"]).toBe("nosniff");
    }
  });
});
