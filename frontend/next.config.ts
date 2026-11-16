import type { NextConfig } from "next";
import { securityHeaders } from "./lib/security-headers";

// The browser only ever calls /api/* on this origin; Next.js forwards those
// requests to the Go API with the /api prefix removed. API_ORIGIN is read on
// the server at build and start time and never reaches the browser.
const apiOrigin = (process.env.API_ORIGIN ?? "http://localhost:8080").replace(/\/$/, "");

const nextConfig: NextConfig = {
  cacheComponents: true,
  images: { formats: ["image/avif", "image/webp"] },
  partialPrefetching: true,
  poweredByHeader: false,
  turbopack: {
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  // Security headers on every page and asset (Stage 17a). The API sets its
  // own on /api/* answers; a stricter API value is kept by the browser
  // because both are sent.
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders({
          dev: process.env.NODE_ENV !== "production",
          turnstile: Boolean(process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY),
        }),
      },
    ];
  },
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiOrigin}/:path*` }];
  },
};

export default nextConfig;
