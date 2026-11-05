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
