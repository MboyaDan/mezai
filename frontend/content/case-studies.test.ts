import { describe, expect, it } from "vitest";
import { publishedCaseStudies, type CaseStudy } from "./case-studies";

const base: CaseStudy = {
  slug: "sample",
  title: "Sample (Sample)",
  client: "Sample Client Ltd (Sample)",
  summary: "s",
  service: "Data annotation",
  sections: [],
  results: [{ label: "Items", value: "1", source: "Delivery manifest" }],
  published: true,
  permission: { reference: "docs/content/permissions/sample.pdf", grantedOn: "2026-01-01" },
};

describe("publishedCaseStudies", () => {
  it("keeps a published record with written permission and sourced results", () => {
    expect(publishedCaseStudies([base])).toHaveLength(1);
  });

  it("drops drafts, records without permission and unsourced results", () => {
    expect(publishedCaseStudies([{ ...base, published: false }])).toEqual([]);
    expect(publishedCaseStudies([{ ...base, permission: null }])).toEqual([]);
    expect(
      publishedCaseStudies([{ ...base, permission: { reference: " ", grantedOn: "2026-01-01" } }]),
    ).toEqual([]);
    expect(publishedCaseStudies([{ ...base, permission: { reference: "x", grantedOn: "soon" } }])).toEqual(
      [],
    );
    expect(
      publishedCaseStudies([{ ...base, results: [{ label: "Accuracy", value: "99%", source: "" }] }]),
    ).toEqual([]);
  });
});
