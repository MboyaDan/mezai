// "How quality works" page copy (brief section 8). Every number here is a
// documented method or a default project setting, never a result or a
// promise. Results only ever come from a real project's dashboard.

export const qualityPage = {
  eyebrow: "How quality works",
  title: { lead: "Many careful judgments.", accent: "One", tail: "verified answer." },
  intro:
    "We do not ask you to trust our people. We measure them, on every project, with checks you can see in your dashboard and in the manifest that ships with each delivery.",
  steps: [
    {
      id: "qualify",
      name: "Qualify",
      line: "Nobody sees a paid task before passing a test for that skill.",
      body: [
        "Each skill has a short qualification test built from items with known answers. It is scored automatically against a pass mark set for that test.",
        "Our operations team then approves or rejects each applicant, with a note. Contributors work only on projects that match the skills they qualified in.",
      ],
    },
    {
      id: "gold",
      name: "Gold checks",
      line: "Items with known answers, mixed in where nobody can spot them.",
      body: [
        "Every project mixes gold items into each contributor's queue at a rate you set (10 percent by default). They look exactly like the other items, so they measure everyday work, not test-day behaviour.",
        "We never reveal a gold result at the moment of submission, because that would show which item was gold. Feedback on misses arrives in a daily batch.",
      ],
    },
    {
      id: "overlap",
      name: "Overlap",
      line: "Each item answered by several people who never see each other's answers.",
      body: [
        "You choose k, the number of different contributors per item, from 1 to 5. Nobody answers the same item twice.",
        "Low-risk work can run at k = 1 with a higher gold rate. Subjective or high-stakes work runs at 3 or more.",
      ],
    },
    {
      id: "agreement",
      name: "Agreement",
      line: "How far people agree beyond chance, per project and per label.",
      body: [
        "We report percent agreement alongside a chance-corrected statistic: Cohen's kappa for two raters, Fleiss' kappa for three or more. For rubric scores we report the mean absolute difference between raters.",
        "Every figure comes with a plain-language band. The band is a guide; the number is always shown.",
      ],
    },
    {
      id: "review",
      name: "Review",
      line: "Disagreements and a random sample go to a senior reviewer.",
      body: [
        "An item goes to review when there is no majority, when scores spread more than the project's threshold (2 points by default), when a contributor flags it, or when it is drawn in a random audit sample (5 percent by default).",
        "The reviewer accepts the consensus, chooses one answer or writes the final one, and can send feedback. Rejections always carry a reason the contributor can read.",
      ],
    },
    {
      id: "deliver",
      name: "Deliver",
      line: "Final answers with a manifest you can check.",
      body: [
        "Deliveries are JSONL and CSV files with a manifest describing exactly what is in them, downloaded through short-lived signed links. Each download is recorded.",
      ],
    },
  ],
  kappa: {
    title: "Agreement, beyond chance",
    intro:
      "Two people label the same 50 messages as spam or not spam. Change how often they agree and watch percent agreement and Cohen's kappa move apart.",
    note: "Worked example with made-up counts. Kappa = (observed agreement − chance agreement) ÷ (1 − chance agreement).",
  },
  bands: [
    { band: "weak", range: "0.40 and below" },
    { band: "moderate", range: "0.41 to 0.60" },
    { band: "substantial", range: "0.61 to 0.80" },
    { band: "strong", range: "above 0.80" },
  ],
  trust: {
    title: "Each contributor's trust score",
    intro:
      "Every contributor has a score between 0 and 1, recalculated after each gold result and each review decision, over the last 30 days.",
    parts: [
      { weight: 0.5, name: "Gold accuracy" },
      { weight: 0.3, name: "Agreement with the final answer" },
      { weight: 0.2, name: "Review acceptance rate" },
    ],
    rules: [
      "Until someone has 20 gold results, their score shows as “building” and they see more gold items.",
      "Below 0.70, a contributor is paused from new tasks automatically and our team is told. Reinstating someone needs a written note.",
      "Contributors see their own score, its three parts and plain advice. You see only your project's overall quality, never individual scores tied to names.",
    ],
    note: "Weights and thresholds are platform settings, shown here at their defaults.",
  },
  manifest: {
    title: "What the manifest records",
    label: "Demonstration",
    caption: "The shape of a delivery manifest. Every value below is fictional.",
    fields: [
      "Project and export time",
      "Schema version",
      "Item counts: total, consensus, adjudicated, rejected",
      "Gold accuracy, for the project and its spread across contributors",
      "Agreement statistics and review rate",
      "Contributor count, as IDs only",
      "A checksum for every file",
    ],
  },
  security: {
    id: "security",
    title: "How we handle your data",
    items: [
      {
        name: "Item by item",
        text: "Contributors see one assigned item at a time, never a whole dataset. They cannot export or download in bulk. Files open through short-lived signed links.",
      },
      {
        name: "Your organisation only",
        text: "Your users see only your organisation's projects, items and deliverables. The database enforces this separation as well as the application.",
      },
      {
        name: "Nothing overwritten",
        text: "Submitted work is never edited in place: a change is a new revision linked to the last. Every change by our team is written to an audit log.",
      },
      {
        name: "Deleted on schedule",
        text: "Task content is deleted or anonymised after delivery, on a schedule set per project.",
      },
      {
        name: "Under contract",
        text: "Every contributor works under contributor terms with confidentiality obligations, and accepts them before seeing any task.",
      },
    ],
    note: "We hold no security certifications yet, and we will not suggest otherwise.",
  },
  cta: {
    title: "See these numbers on your own data.",
    primary: { label: "Start a pilot project", href: "/contact" },
  },
};
