// Trust page copy (Stage 21a, D-161). Every statement describes something the
// platform does today and a test or the code shows it (see docs/security.md).
// Commitments ELS has not confirmed in writing (NDA, DPA, ODPC status,
// fair-pay policy) are empty here and the page hides them until
// docs/content/ has the answer. No certification is claimed: there is none.

import { brand } from "@/lib/brand";

export type TrustPoint = { name: string; text: string };

export const trustPage = {
  eyebrow: "Trust and data",
  title: { lead: "How we look after", accent: "your data." },
  intro: `What happens to your files from upload to delivery, who can see them, and how the people doing the work are chosen. We describe what the platform does today, not what we plan to do.`,

  handling: {
    title: "Your data",
    points: [
      {
        name: "Kept apart from other clients",
        text: "Each client's projects, items and deliveries are tied to their organisation. The application checks this on every request, and the database enforces it again with row-level security, so one client cannot read another's data even through a faulty screen. Automated tests try exactly that on every change.",
      },
      {
        name: "Seen one item at a time",
        text: "Contributors see only the item they are working on, one at a time. They never see the whole dataset, cannot export it, and files open through links that expire within minutes.",
      },
      {
        name: "Deleted after delivery",
        text: "Task content and answers are deleted 90 days after a project closes. Counts, quality figures and the record of who did what are kept; item text, answers and files are not.",
      },
      {
        name: "Every change on record",
        text: "Imports, reviews, deliveries and payments are written to an audit log. Your team sees your organisation's activity in your workspace.",
      },
      {
        name: "Delivered with a manifest",
        text: "Each delivery comes as JSONL and CSV with a manifest of counts, gold accuracy, agreement and checksums. Download links last five minutes and every download is recorded.",
      },
    ] satisfies TrustPoint[],
    link: { label: "See a sample delivery and its manifest", href: "/proof" },
  },

  access: {
    title: "Who can get in",
    points: [
      {
        name: "Permissions, not job titles",
        text: "Every action needs a named permission, checked on the server. Clients see only their own organisation; reviewers see only the projects they are assigned to.",
      },
      {
        name: "Sign-in",
        text: "Passwords are stored as Argon2id hashes. Repeated wrong passwords lock the account for a while. Staff can turn on two-step sign-in with an authenticator app.",
      },
      {
        name: "Contributors stay private",
        text: "Your workspace shows contributors as codes, a different code for each client. Phone numbers are masked on screen and in logs, and ID numbers are kept only as a keyed fingerprint and the last three digits.",
      },
    ] satisfies TrustPoint[],
  },

  people: {
    title: "Who does the work",
    intro:
      "Every contributor goes through the same steps before they see a paid task, and keeps being checked afterwards.",
    steps: [
      {
        name: "Application",
        text: "Languages, dialects, domains and experience. The operations team reads every application and shortlists, waitlists or declines it with a reason.",
      },
      {
        name: "Core check",
        text: "Reading, judgment and a short writing sample that a person rates before anyone is approved.",
      },
      {
        name: "Phone interview",
        text: "A short structured call with a fixed rubric: language, a work scenario, and a promise to keep task content private.",
      },
      {
        name: "Identity",
        text: "One person, one account: each M-Pesa number and ID number can belong to only one contributor, and possible duplicates are checked by a person.",
      },
      {
        name: "Skill tests",
        text: "A test for each kind of task, built from gold items with known answers. Skills last 12 months, then are tested again.",
      },
      {
        name: "Checked while working",
        text: "Hidden gold items, agreement with other contributors and review decisions make up a trust score. More gold is mixed in for someone new to a project, and anyone who falls below the floor is paused automatically.",
      },
    ] satisfies TrustPoint[],
    scale:
      "We add people by language and skill when a project needs them, through the same steps. We will publish how many contributors are active once the pilot has real numbers.",
    link: { label: "How quality is measured", href: "/quality" },
  },

  // Answers from ELS, filled from docs/content/ when confirmed. Empty: hidden.
  commitments: {
    title: "Agreements and policies",
    items: [
      { name: "Non-disclosure agreement", text: "" },
      { name: "Data processing agreement", text: "" },
      { name: "Registration with Kenya's Office of the Data Protection Commissioner", text: "" },
      { name: "Fair pay for contributors", text: "" },
    ] satisfies TrustPoint[],
  },

  limits: {
    title: "What we do not claim",
    text: `${brand.name} is in its pilot. We hold no security certification and have not had an external security audit yet. Personal data is handled under Kenya's Data Protection Act, 2019: ${brand.legalEntity} is the data controller for the personal data it collects, and for your task data it acts on your instructions.`,
  },

  cta: {
    primary: { label: "Ask about your data", href: "/contact" },
    secondary: { label: "Read the privacy notice", href: "/legal/privacy" },
  },
};
