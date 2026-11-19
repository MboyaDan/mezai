// About page copy (brief sections 1 to 3). Honest pilot framing: we say what
// exists and what is still being built. No team size, founding story details,
// clients or partners beyond what the brief states.

import { brand } from "@/lib/brand";

export const aboutPage = {
  eyebrow: "About",
  title: { lead: "Meza is Kiswahili for", accent: "table." },
  intro: `The place where people sit down and work something out together. That is the job: many careful people, one answer you can rely on.`,
  story: [
    `${brand.name} is ${brand.descriptor}, a software company in ${brand.city}. We run paid data projects for teams building AI, with vetted African contributors, starting in Kenya.`,
    "The market for human AI data has moved from cheap labelling towards evaluation, reasoning and specialist judgment. We do not compete on the lowest price. We compete on expertise that is hard to find elsewhere, African languages and African contexts, and on quality you can check for yourself.",
  ],
  beliefs: {
    title: "What we hold ourselves to",
    items: [
      {
        name: "Show the checks",
        text: "Gold accuracy, agreement and review decisions belong in your dashboard, not in a sales deck.",
      },
      {
        name: "Pay people properly",
        text: "Contributors are paid per approved task, on record, through M-Pesa, with a reason for every rejection.",
      },
      {
        name: "Claim only what is true",
        text: "No borrowed logos, no invented numbers. When we have results, they will come with a source.",
      },
    ],
  },
  // Supplied and approved by Dan, with permission for the photo
  // (docs/content/founder.md, 2026-11-19). Use as written.
  founder: {
    title: "Who runs it",
    name: "Dan Roylex",
    role: `Founder, ${brand.name}`,
    org: `${brand.descriptor.charAt(0).toUpperCase()}${brand.descriptor.slice(1)}`,
    linkedin: { label: "Dan Roylex on LinkedIn", href: "https://www.linkedin.com/in/danroylex" },
    photoAlt: "Dan Roylex, smiling, in a dark T-shirt against a light curtain.",
    bio: [
      `I’m a backend engineer based in Nairobi. Since 2018 I’ve run ${brand.legalEntity}, building production systems in Go and Python for clients in fintech, e-commerce, SaaS and restaurant tech. Much of that work has been payment infrastructure: M-Pesa integrations, webhook verification, and making sure a transaction is recorded exactly once even when the network isn’t cooperating. It’s taught me how East African customers actually write and talk to the systems they use: in Kiswahili, in English, in Sheng, often all three in one message.`,
      `I started ${brand.name} because the AI tools now reaching Kenyan users are mostly tested by people who don’t speak the way those users do. A model can pass every English benchmark and still misread “pesa haijaingia.” ${brand.name} gives AI teams human evaluation and labelled data from qualified native speakers, with the quality measured and reported on every batch rather than just promised. I run the process myself, from rubric to delivery, and I work directly with every client.`,
    ],
  },
  pilot: {
    title: "Where we are",
    text: "We are in our pilot: running first projects with a small group of contributors, on a platform we built for this work. If you join now, you work with the people who built it, and you help shape what comes next.",
  },
  entity: {
    title: "Who you contract with",
    text: `Contracts, invoices, the privacy notice and the contributor terms are with ${brand.legalEntity}, ${brand.city}, Kenya.`,
  },
  photo: {
    alt: "Seen from above, two people work on laptops at a wooden table with an open notebook and a coffee between them.",
  },
  cta: {
    primary: { label: "Start a pilot project", href: "/contact" },
    secondary: { label: "Join as a contributor", href: "/contributors" },
  },
};
