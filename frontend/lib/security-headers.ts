// Response headers for every page (Stage 17a, D-138). Kept as a pure
// function so the policy is tested; next.config.ts applies it.
//
// The content security policy allows only this origin. Scripts and styles
// may be inline because Next.js puts its hydration data in inline scripts
// and the motion library writes inline styles; a nonce would force every
// page to render per request and lose the static public pages (A-081).
// What the policy does stop: scripts, frames, forms, fonts and connections
// to or from any other origin, plugins, and <base> tricks.

export type HeaderOptions = {
  /** `next dev` needs eval and a websocket for hot reload. */
  dev: boolean;
  /** Cloudflare Turnstile is loaded only when its site key is set (DG-07). */
  turnstile: boolean;
};

const TURNSTILE = "https://challenges.cloudflare.com";

export function contentSecurityPolicy({ dev, turnstile }: HeaderOptions): string {
  const script = [
    "'self'",
    "'unsafe-inline'",
    ...(dev ? ["'unsafe-eval'"] : []),
    ...(turnstile ? [TURNSTILE] : []),
  ];
  const connect = ["'self'", ...(dev ? ["ws:"] : [])];
  const directives: Record<string, string[]> = {
    "default-src": ["'self'"],
    "script-src": script,
    "style-src": ["'self'", "'unsafe-inline'"],
    "img-src": ["'self'", "data:", "blob:"],
    "font-src": ["'self'"],
    "connect-src": connect,
    "frame-src": turnstile ? [TURNSTILE] : ["'none'"],
    "frame-ancestors": ["'none'"],
    "form-action": ["'self'"],
    "base-uri": ["'self'"],
    "object-src": ["'none'"],
    "manifest-src": ["'self'"],
    "worker-src": ["'self'", "blob:"],
  };
  return Object.entries(directives)
    .map(([name, values]) => `${name} ${values.join(" ")}`)
    .join("; ");
}

export function securityHeaders(options: HeaderOptions): Array<{ key: string; value: string }> {
  const headers = [
    { key: "Content-Security-Policy", value: contentSecurityPolicy(options) },
    { key: "X-Content-Type-Options", value: "nosniff" },
    { key: "X-Frame-Options", value: "DENY" },
    { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
    { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
    { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=(), payment=(), usb=()" },
  ];
  // Browsers ignore HSTS on plain HTTP, and localhost must stay reachable.
  if (!options.dev) {
    headers.push({ key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" });
  }
  return headers;
}
