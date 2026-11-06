import { NextResponse, type NextRequest } from "next/server";

// Signed-in areas: with no session cookie at all, go to sign-in first instead
// of loading the portal and bouncing. The API still checks every request; a
// stale cookie is handled by the page (RequireSession).
export function proxy(request: NextRequest) {
  if (request.cookies.has("mezai_session")) return NextResponse.next();
  const url = request.nextUrl.clone();
  url.pathname = "/login";
  url.search = `?next=${encodeURIComponent(request.nextUrl.pathname)}`;
  return NextResponse.redirect(url);
}

export const config = { matcher: ["/c", "/c/:path*", "/w", "/w/:path*"] };
