import type { MetadataRoute } from "next";
import { publicRoutes, siteUrl } from "@/lib/site";
import { publishedCaseStudies } from "@/content/case-studies";

// Case studies are listed only once one is published with permission.
export default function sitemap(): MetadataRoute.Sitemap {
  const studies = publishedCaseStudies();
  const paths = [
    ...publicRoutes,
    ...(studies.length > 0 ? ["/case-studies", ...studies.map((c) => `/case-studies/${c.slug}`)] : []),
  ];
  return paths.map((path) => ({
    url: `${siteUrl}${path}`,
    changeFrequency: "monthly",
    priority: path === "/" ? 1 : 0.7,
  }));
}
