// Services page copy (brief section 6.1 and 7). Each service describes the
// task type the platform runs and the checks applied to it. No prices
// (brief 2.1), no volumes, no client names.

import type { ServiceKey } from "@/content/home";

export type Service = {
  key: ServiceKey;
  /** Anchor from docs/design/sitemap.md. */
  id: string;
  name: string;
  summary: string;
  /** What the contributor does on each item. */
  task: string[];
  /** What you receive per item. */
  deliver: string[];
  /** How the quality engine applies to this service. */
  checks: string[];
  goodFor: string;
};

export const servicesPage = {
  eyebrow: "Services",
  title: { lead: "Careful human judgment,", accent: "measured", tail: "item by item." },
  intro:
    "Every project runs on the same platform: vetted contributors, gold checks, overlap, agreement and review. You choose the kind of work; the checks come with it.",
  items: [
    {
      key: "evaluation",
      id: "evaluation",
      name: "AI evaluation and RLHF",
      summary:
        "Contributors compare two model responses to the same prompt, score each against your rubric and say which is better, and why.",
      task: [
        "Read a prompt and two responses, side by side.",
        "Score each response on your rubric, for example accuracy, helpfulness, safety and language quality, 1 to 5.",
        "Choose A, B or a tie, and write a rationale of at least the length you set.",
      ],
      deliver: [
        "Rubric scores per response from each contributor.",
        "The majority preference, the mean score and its spread.",
        "Every written rationale, and the reviewer's decision where there was one.",
      ],
      checks: [
        "Gold comparisons with a known preference: the preference must match and each score fall within 1 point.",
        "Items whose scores spread more than your threshold go to a reviewer.",
      ],
      goodFor: "Preference data, model comparisons and evaluating how a model handles African contexts.",
    },
    {
      key: "language",
      id: "african-languages",
      name: "African language data",
      summary:
        "Fluent speakers review translations and text in Kiswahili and English, including code-switched text, and correct what is wrong.",
      task: [
        "Read the source text and a candidate translation, human or machine.",
        "Score adequacy (is the meaning kept?) and fluency (does it read naturally?), 1 to 5.",
        "Write a corrected translation and tag the kinds of error found.",
      ],
      deliver: [
        "Adequacy and fluency scores from each contributor, with the mean and spread.",
        "The corrected translation chosen at review when contributors differ.",
        "Error tags per item.",
      ],
      checks: [
        "Gold items where adequacy and fluency must land within 1 point of the known scores.",
        "Corrections are kept as revisions: earlier answers are never overwritten.",
      ],
      goodFor: "Machine translation evaluation, chatbot localisation and language dataset projects.",
    },
    {
      key: "annotation",
      id: "annotation",
      name: "Data annotation",
      summary:
        "Contributors label text with your label set, single or multi-label, in any language we cover, with an optional confidence and note.",
      task: [
        "Read one text at a time.",
        "Choose the label, or every label that applies, from your set.",
        "Add a confidence and a note when the text is unclear.",
      ],
      deliver: [
        "The final label per item, by majority vote, or by a reviewer where there was no majority.",
        "Agreement per project and per label: percent agreement with Cohen's or Fleiss' kappa.",
      ],
      checks: [
        "Gold items scored by exact match (single label) or a Jaccard similarity of at least 0.8 (multi-label).",
        "Ties go to review.",
      ],
      goodFor: "Intent and topic classification, content moderation labels, sentiment and routing data.",
    },
    {
      key: "review",
      id: "expert-review",
      name: "Expert review",
      summary:
        "Senior reviewers look at every submission for an item side by side and decide the final answer, with a reason on record.",
      task: [
        "Compare all submissions for one item, with each contributor's ID and trust score.",
        "Accept the consensus, choose one submission or write the final answer.",
        "Reject with a reason, which the contributor sees, and send feedback.",
      ],
      deliver: [
        "A decision on record for every item that needed one.",
        "The review rate in your project dashboard and delivery manifest.",
      ],
      checks: [
        "Items go to review when there is no majority, scores spread too far, a contributor flags the item, or a random audit sample is drawn.",
      ],
      goodFor: "Adjudicating hard cases, auditing a sample of finished work, and domain-specialist checks.",
    },
  ] satisfies Service[],
  /** Every project, whatever the service. */
  common: {
    title: "On every project",
    items: [
      "Your instructions and rubric, written in the workspace and shown to contributors with each task.",
      "A live dashboard: progress against target, gold accuracy, agreement and review rate.",
      "Delivery as JSONL and CSV with a manifest: counts, agreement, gold accuracy, export time and a checksum per file.",
    ],
    link: { label: "How quality works", href: "/quality" },
  },
};

export type LanguageStatus = "available" | "on-request";

/**
 * Language capability (brief Q1). Filled only from confirmed content. Until
 * Dan confirms a list in docs/content/, the brief's default applies (A-001):
 * English and Kiswahili available, everything else on request. Code-switched
 * Kiswahili and English mixes the two, as the home page already says (A-029).
 */
export const languages = {
  title: "Languages",
  intro:
    "We list a language as available only when we can staff and check it today. Anything else, ask us: we will tell you honestly whether we can.",
  rows: [
    { name: "English", status: "available" as LanguageStatus },
    { name: "Kiswahili", status: "available" as LanguageStatus },
    { name: "Code-switched Kiswahili and English", status: "available" as LanguageStatus },
    { name: "Other African languages", status: "on-request" as LanguageStatus },
  ],
  statusLabel: { available: "Available", "on-request": "On request" } satisfies Record<
    LanguageStatus,
    string
  >,
  basis: "We add a language when we have qualified contributors and a reviewer for it.",
  cta: { label: "Ask about a language", href: "/contact" },
};
