import { describe, expect, it } from "vitest";

import { canonicalRedirect } from "./canonical-host";

const site = "https://mezai.vercel.app";
const at = (u: string): [string, string] => {
  const url = new URL(u);
  return [url.host, url.pathname + url.search];
};

describe("canonicalRedirect", () => {
  it("sends a production deployment address to the site, keeping path and query", () => {
    expect(
      canonicalRedirect(...at("https://mezai-abc123-team.vercel.app/join?x=1"), site, "production")?.href,
    ).toBe("https://mezai.vercel.app/join?x=1");
  });

  it("leaves the site's own address alone", () => {
    expect(canonicalRedirect(...at("https://mezai.vercel.app/join"), site, "production")).toBeNull();
  });

  it("leaves previews and local runs alone", () => {
    expect(canonicalRedirect(...at("https://mezai-git-x-team.vercel.app/"), site, "preview")).toBeNull();
    expect(canonicalRedirect(...at("http://localhost:3000/"), "http://localhost:3000", undefined)).toBeNull();
  });

  it("never redirects a custom domain or when SITE_URL is missing or invalid", () => {
    expect(canonicalRedirect(...at("https://mezai.example/"), site, "production")).toBeNull();
    expect(canonicalRedirect(...at("https://mezai-abc.vercel.app/"), undefined, "production")).toBeNull();
    expect(canonicalRedirect(...at("https://mezai-abc.vercel.app/"), "not a url", "production")).toBeNull();
  });
});
