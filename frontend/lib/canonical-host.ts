// The live site answers on one address (SITE_URL). Vercel also serves each
// production deployment on its own *.vercel.app address; forms sent from
// there fail the API's origin check (APP_ORIGIN), which looks like a broken
// site. So on the production deployment, any other *.vercel.app host is
// redirected to SITE_URL. Previews (VERCEL_ENV=preview) and local runs are
// left alone.
// host is the address the browser asked for (x-forwarded-host or Host): Next's
// nextUrl reports the server's own address, not the visitor's.
export function canonicalRedirect(
  host: string | null,
  pathAndQuery: string,
  siteUrl: string | undefined,
  vercelEnv: string | undefined,
): URL | null {
  if (vercelEnv !== "production" || !siteUrl || !host) return null;
  let site: URL;
  try {
    site = new URL(siteUrl);
  } catch {
    return null;
  }
  const hostname = host.split(":")[0].toLowerCase();
  if (host.toLowerCase() === site.host || !hostname.endsWith(".vercel.app")) return null;
  return new URL(pathAndQuery, site.origin);
}
