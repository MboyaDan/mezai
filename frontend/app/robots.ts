import type { MetadataRoute } from "next";
import { indexable, siteUrl } from "@/lib/site";

export default function robots(): MetadataRoute.Robots {
  if (!indexable) {
    return { rules: { userAgent: "*", disallow: "/" } };
  }
  return {
    // "$" ends a rule so "/c" does not also block /contact or /contributors.
    rules: { userAgent: "*", allow: "/", disallow: ["/api/", "/design", "/c$", "/c/", "/w$", "/w/"] },
    sitemap: `${siteUrl}/sitemap.xml`,
  };
}
