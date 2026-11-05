// "For contributors" page copy (brief 6.1, 6.2, 8.4 and 13). No pay figures
// (Q3: set per project), no contributor counts, no promises of volume.

export const contributorsPage = {
  eyebrow: "For contributors",
  title: { lead: "Paid for careful work.", accent: "Measured", tail: "fairly." },
  intro:
    "MezAI pays vetted people, starting in Kenya, to review, label and evaluate data for AI teams. If you read closely and care about getting the answer right, we would like to hear from you.",
  join: { label: "Apply to join", href: "/join" },
  signIn: { label: "Already a contributor? Sign in", href: "/login" },
  lookFor: {
    title: "Who we look for",
    items: [
      {
        name: "Fluent speakers",
        text: "Native or fluent in Kiswahili and English, including the mixed way people really write.",
      },
      { name: "Domain specialists", text: "Health, law, finance, education, software or agriculture." },
      {
        name: "Careful readers",
        text: "People who follow instructions closely and notice when an answer only sounds right.",
      },
    ],
    basics: [
      "A phone or computer with a reliable internet connection",
      "An M-Pesa number in your name, for payments",
      "A few hours a week; you choose when",
    ],
  },
  steps: {
    title: "From application to first task",
    items: [
      {
        name: "Apply",
        text: "Create an account, confirm your email and accept the privacy notice and contributor terms.",
      },
      {
        name: "Build your profile",
        text: "Your languages and how well you know them, any dialects, your domains, education and weekly availability.",
      },
      {
        name: "Qualify",
        text: "Take a short test for each skill. It is scored automatically, then our team reviews your result.",
      },
      {
        name: "Work",
        text: "Projects that match your skills appear on your dashboard with the pay per task, shown before you start.",
      },
    ],
  },
  pay: {
    title: "How pay works",
    items: [
      {
        name: "Per approved task",
        text: "Each project sets a pay rate per task. You see it before you start.",
      },
      {
        name: "In shillings, by M-Pesa",
        text: "Approved work is paid to your M-Pesa number in Kenyan shillings, in regular payout runs.",
      },
      {
        name: "Every shilling on record",
        text: "Your dashboard shows what is pending, approved and paid, line by line, with each M-Pesa receipt.",
      },
      {
        name: "Reasons, not silence",
        text: "If a task is rejected you see the reason, so you know what to do differently.",
      },
    ],
  },
  measured: {
    title: "How you are measured",
    text: "Some items have known answers. Your trust score combines how often you match them, how often you agree with the final answer and how often reviewers accept your work. You can always see your score, what goes into it and how to improve it.",
    link: { label: "How quality works", href: "/quality#trust" },
  },
  terms: {
    title: "The terms, in brief",
    items: [
      "You work as an independent contractor under our contributor terms, not as an employee.",
      "Task content is confidential. You see one item at a time and may not copy, share or download it.",
      "You can skip a task with a reason, and report content that is harmful or upsetting.",
      "If your trust score drops below the floor, you are paused from new tasks and a person reviews it. You are not removed automatically.",
      "We collect only the personal data we need, and we do not ask for ID documents in the pilot.",
    ],
    link: { label: "Read the contributor terms", href: "/legal/contributor-terms" },
    note: "This summary is not the agreement. The contributor terms are what you accept.",
  },
  photos: {
    room: {
      alt: "Four people work at laptops on white desks in a bright office, some wearing headsets.",
    },
    phone: {
      alt: "A hand holds a smartphone showing a grid of photos, with a laptop at the edge of the frame.",
    },
  },
};
