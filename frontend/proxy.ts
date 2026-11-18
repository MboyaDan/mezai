import { NextResponse, type NextRequest } from "next/server";

import { canonicalRedirect } from "@/lib/canonical-host";
import { apiRequestHeaders } from "@/lib/client-ip";

export function proxy(request: NextRequest) {
  const canonical = canonicalRedirect(
    request.headers.get("x-forwarded-host") ?? request.headers.get("host"),
    request.nextUrl.pathname + request.nextUrl.search,
    process.env.SITE_URL,
    process.env.VERCEL_ENV,
  );
  if (canonical) return NextResponse.redirect(canonical, 308);

  const path = request.nextUrl.pathname;
  // /api is rewritten to the Go API (next.config.ts): pass the visitor's
  // address along with the shared secret (D-156).
  if (path.startsWith("/api/")) {
    return NextResponse.next({
      request: { headers: apiRequestHeaders(request.headers, process.env.PROXY_SHARED_SECRET) },
    });
  }
  // Signed-in areas: with no session cookie at all, go to sign-in first instead
  // of loading the portal and bouncing. The API still checks every request; a
  // stale cookie is handled by the page (RequireSession).
  const portal = path === "/c" || path.startsWith("/c/") || path === "/w" || path.startsWith("/w/");
  if (!portal || request.cookies.has("mezai_session")) return NextResponse.next();
  const url = request.nextUrl.clone();
  url.pathname = "/login";
  url.search = `?next=${encodeURIComponent(path)}`;
  return NextResponse.redirect(url);
}

// Every page and /api; not Next's own files or static assets (anything with a
// file extension), which need no redirect and stay fast.
export const config = { matcher: ["/((?!_next/|.*\\.[a-zA-Z0-9]+$).*)"] };
