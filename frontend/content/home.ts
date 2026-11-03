// Home page copy (DESIGN.md section 10). Short, concrete, no invented
// claims: no client names, counts, accuracy figures or certifications.
// Anything not yet true is phrased as how we work or what we look for.

export const hero = {
  eyebrow: "AI data operations · Nairobi",
  headline: { lead: "Human judgment,", accent: "verified.", second: "Data for AI, built in Africa." },
  subline:
    "We run evaluation, African-language and annotation projects with vetted contributors, and show you the quality checks behind every item we deliver.",
  primary: { label: "Start a pilot project", href: "/contact" },
  secondary: { label: "Join as a contributor", href: "/contributors" },
  latticeCaption: ["Many judgments.", "One verified answer."],
};

export const network = {
  eyebrow: "The connection",
  title: "Expertise where it lives. Models where they are built.",
  lines: [
    "Fluent speakers, domain specialists and careful reviewers work from Nairobi and other African cities.",
    "The teams building and testing AI models are spread across the world.",
    "MezAI runs the work in between, and every answer passes the same checks before it reaches you.",
  ],
  note: "Illustrative. Cities show the idea, not current project locations.",
};

export type ServiceKey = "evaluation" | "language" | "annotation" | "review";

export const services = {
  eyebrow: "What we do",
  title: { lead: "Four kinds of work. One", accent: "quality engine", tail: "behind all of them." },
  items: [
    {
      key: "evaluation" as ServiceKey,
      name: "AI evaluation and RLHF",
      line: "Side-by-side comparisons of model responses, scored against your rubric, with a written reason for every preference.",
    },
    {
      key: "language" as ServiceKey,
      name: "African language data",
      line: "Kiswahili and code-switched text written, translated and reviewed by fluent speakers, with adequacy and fluency scored per item.",
    },
    {
      key: "annotation" as ServiceKey,
      name: "Data annotation",
      line: "Text classification, single or multi-label, with gold checks mixed in and every item answered by more than one person.",
    },
    {
      key: "review" as ServiceKey,
      name: "Expert review",
      line: "Senior reviewers resolve disagreements, write the final answer and send feedback, so every delivered item has a decision on record.",
    },
  ],
  industries: {
    label: "Built for",
    items: [
      "AI labs and model builders",
      "Telcos and banks",
      "Health",
      "Education and research",
      "Public sector and NGOs",
    ],
    link: { label: "Industries", href: "/industries" },
  },
};

// No photos until docs/content holds credited images (DESIGN.md section 8):
// this frame states who we look for, never who "our contributors" are.
export const people = {
  eyebrow: "The people",
  title: "Vetted by test, measured by results.",
  intro:
    "We look for people who already do this work with care, then ask them to prove it on a short qualification test before they see a paid task.",
  profiles: [
    {
      name: "Linguists and translators",
      line: "Native and fluent speakers of Kiswahili, English and code-switched Sheng.",
    },
    { name: "Engineers", line: "Developers who can judge whether code and technical answers actually work." },
    { name: "Clinicians", line: "Health workers who can tell a safe answer from a confident wrong one." },
    { name: "Teachers", line: "Educators who explain clearly and spot an answer that only sounds right." },
  ],
  steps: [
    "Apply with your languages, domains and availability.",
    "Pass a qualification test built from items with known answers.",
    "Work on projects that match your skills; your accuracy and agreement are measured on every task.",
  ],
  link: { label: "How contributing works", href: "/contributors" },
};

export const demo = {
  eyebrow: "See it work",
  title: "One item, from a contributor's screen to a client's report.",
  label: "Demonstration",
  note: "Fictional data. This shows the product's design; it is not a live project.",
  steps: [
    {
      title: "A contributor reviews a translation",
      caption:
        "The source and the machine translation sit side by side. The contributor scores adequacy and fluency from 1 to 5.",
    },
    {
      title: "They correct one phrase and submit",
      caption: "The corrected translation is saved as a new revision. Earlier answers are never overwritten.",
    },
    {
      title: "A reviewer checks it",
      caption:
        "Three contributors answered the same item. Their scores were close, and a reviewer accepts the consensus.",
    },
    {
      title: "Agreement updates",
      caption:
        "The project's agreement figure is recalculated with the new item, shown as a number and a plain-language band.",
    },
    {
      title: "The client sees progress and quality",
      caption:
        "The client view shows items completed against target, gold accuracy, agreement and the review rate.",
    },
  ],
};

export const quality = {
  eyebrow: "How quality works",
  title: { lead: "The method,", accent: "in the open." },
  steps: [
    { name: "Qualify", line: "A short test per skill, scored against known answers." },
    {
      name: "Gold checks",
      line: "Items with known answers mixed into every queue, indistinguishable from the rest.",
    },
    { name: "Overlap", line: "Each item answered by several people who never see each other's answers." },
    {
      name: "Agreement",
      line: "Cohen's or Fleiss' kappa with percent agreement, reported per project and per label.",
    },
    { name: "Review", line: "Disagreements and a random sample go to a senior reviewer." },
    {
      name: "Deliver",
      line: "Final answers with a manifest: counts, agreement, gold accuracy and checksums.",
    },
  ],
  definitions: [
    {
      term: "Gold item",
      text: "An item whose correct answer we already know. Contributors cannot tell it apart, so it measures their accuracy honestly.",
    },
    {
      term: "Kappa",
      text: "How much people agree beyond what chance alone would produce. 1 is perfect agreement; 0 is no better than chance.",
    },
  ],
  calculator: {
    title: "Try it: three people label one message",
    intro: "Choose each person's label. With no majority, the item goes to review.",
    labels: ["Billing", "Network", "Other"],
  },
  link: { label: "How quality works in full", href: "/quality" },
};

// Only practices the platform implements or the brief commits to (section 12).
export const practices = {
  title: "How we handle your data",
  items: [
    {
      text: "Data stays on our platform. Contributors see one assigned item at a time, never a whole dataset.",
      link: { label: "Security", href: "/quality#security" },
    },
    {
      text: "Every contributor works under contributor terms with confidentiality obligations.",
      link: { label: "Contributor terms", href: "/legal/contributor-terms" },
    },
    {
      text: "Task content is deleted or anonymised after delivery, on a schedule you set.",
      link: { label: "Privacy notice", href: "/legal/privacy" },
    },
  ],
};

export const finalCta = {
  title: "Run a pilot with us.",
  primary: { label: "Start a pilot project", href: "/contact" },
  secondary: { label: "Join as a contributor", href: "/contributors" },
};
