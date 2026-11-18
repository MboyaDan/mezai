import { NextResponse, type NextRequest } from "next/server";

import { apiRequestHeaders } from "@/lib/client-ip";

export function proxy(request: NextRequest) {
  // /api is rewritten to the Go API (next.config.ts): pass the visitor's
  // address along with the shared secret (D-156).
  if (request.nextUrl.pathname.startsWith("/api/")) {
    return NextResponse.next({
      request: { headers: apiRequestHeaders(request.headers, process.env.PROXY_SHARED_SECRET) },
    });
  }
  // Signed-in areas: with no session cookie at all, go to sign-in first instead
  // of loading the portal and bouncing. The API still checks every request; a
  // stale cookie is handled by the page (RequireSession).
  if (request.cookies.has("mezai_session")) return NextResponse.next();
  const url = request.nextUrl.clone();
  url.pathname = "/login";
  url.search = `?next=${encodeURIComponent(request.nextUrl.pathname)}`;
  return NextResponse.redirect(url);
}

export const config = { matcher: ["/api/:path*", "/c", "/c/:path*", "/w", "/w/:path*"] };
