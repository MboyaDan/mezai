// Proof page copy (Stage 21b, D-162). The package on the page is fictional
// and says so; its files come from the same export code as a real delivery
// (lib/proof.ts). Nothing here describes a real client or real results.

export const proofPage = {
  eyebrow: "Sample delivery",
  title: { lead: "What you receive", accent: "at the end of a project." },
  intro:
    "A complete delivery package you can download and open. The project, client and answers are invented, but the files were made by the same code that builds real deliveries, so you see exactly what arrives.",

  label: "Demonstration",
  disclaimer:
    "Every message, answer and figure in this package is fictional. The figures show how a delivery reports quality; they say nothing about the quality of real work.",

  files: {
    title: "Three files",
    intro:
      "Each delivery version has the same data in two formats and a manifest that describes it. Download links in your workspace last five minutes and every download is recorded; these sample files are public.",
    kinds: {
      manifest: {
        name: "Manifest",
        text: "Counts, gold accuracy, agreement, review rate, contributor codes and a checksum for each data file.",
      },
      jsonl: {
        name: "JSONL",
        text: "One record per line with the item, the final answer and how it was reached. Exact, for pipelines.",
      },
      csv: {
        name: "CSV",
        text: "The same records as columns, for spreadsheets. Cells that a spreadsheet could run as a formula are made safe.",
      },
    },
  },

  manifest: {
    title: "What the manifest says",
    intro:
      "The figures below are read straight from the sample manifest. In a real delivery they come from your project's own answers.",
    items: "Items",
    gold: "Gold accuracy",
    agreement: "Agreement",
    review: "Sent to review",
    perLabel: "Agreement by label",
    reasons: {
      no_majority: "no majority",
      spread: "scores too far apart",
      flagged: "flagged by a contributor",
      audit_sample: "random audit",
    } as Record<string, string>,
  },

  records: {
    title: "The data, item by item",
    intro:
      "Each item keeps your own ID, the final label, whether the answers agreed or a reviewer decided, and which contributors answered, by code only.",
    decidedBy: {
      consensus: "Consensus",
      review: "Reviewer",
      correction: "Correction",
    } as Record<string, string>,
  },

  verify: {
    title: "Check it yourself",
    text: "The manifest records a SHA-256 checksum for each data file. After downloading, compute the checksum and compare it with the manifest. If they match, the file is the one we delivered, unchanged.",
    command: "shasum -a 256",
    notesTitle: "Notes in every manifest",
  },

  cta: {
    title: "Want this for your own data?",
    primary: { label: "Start a pilot", href: "/contact" },
    secondary: { label: "How quality is measured", href: "/quality" },
  },
};
