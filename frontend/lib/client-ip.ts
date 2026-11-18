// The visitor's address for the API (D-156). The API only believes it with
// PROXY_SHARED_SECRET, which stays on the server (never NEXT_PUBLIC_).
export const CLIENT_IP_HEADER = "x-mezai-client-ip";
export const PROXY_TOKEN_HEADER = "x-mezai-proxy-token";

/**
 * Headers for an /api request: copies a visitor tried to send are removed,
 * then the address and secret are set when the secret is configured. Vercel
 * sets x-real-ip and x-forwarded-for itself and overwrites what the visitor
 * sent, so the first address there is the visitor.
 */
export function apiRequestHeaders(incoming: Headers, secret: string | undefined): Headers {
  const headers = new Headers(incoming);
  headers.delete(CLIENT_IP_HEADER);
  headers.delete(PROXY_TOKEN_HEADER);
  if (!secret) return headers;
  const ip =
    incoming.get("x-real-ip")?.trim() || incoming.get("x-forwarded-for")?.split(",")[0]?.trim() || "";
  if (ip) {
    headers.set(CLIENT_IP_HEADER, ip);
    headers.set(PROXY_TOKEN_HEADER, secret);
  }
  return headers;
}
