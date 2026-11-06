// Contact / Start a pilot (brief 6.1). The form fields are the brief's list.
// Option keys must match backend/internal/httpapi/enquiries.go (contact.test.ts checks).
// No reply-time promise and no phone number: neither is confirmed (Q2).

export type Option = { value: string; label: string };

export const projectTypes: Option[] = [
  { value: "ai-evaluation", label: "AI evaluation and RLHF" },
  { value: "african-language-data", label: "African language data" },
  { value: "data-annotation", label: "Data annotation" },
  { value: "expert-review", label: "Expert review" },
  { value: "not-sure", label: "Not sure yet" },
];

export const volumes: Option[] = [
  { value: "under-1k", label: "Under 1,000 items" },
  { value: "1k-10k", label: "1,000 to 10,000 items" },
  { value: "10k-100k", label: "10,000 to 100,000 items" },
  { value: "over-100k", label: "More than 100,000 items" },
  { value: "not-sure", label: "Not sure yet" },
];

export const timelines: Option[] = [
  { value: "1-month", label: "Within a month" },
  { value: "1-3-month", label: "In one to three months" },
  { value: "later", label: "Later than three months" },
  { value: "exploring", label: "Just exploring" },
];

/** Field limits, the same as the API's. */
export const limits = { name: 120, organisation: 200, languages: 200, message: 5000 } as const;

export const contactPage = {
  eyebrow: "Start a pilot",
  title: { lead: "Tell us about", accent: "the data." },
  intro:
    "A pilot starts small: one task type, a few hundred items, gold questions you can check. Tell us what you need and we will reply by email.",
  form: {
    title: "Project enquiry",
    submit: "Send enquiry",
    privacy:
      "We use these details only to reply to you and plan the pilot. They are not shared with contributors or sold.",
    privacyLink: { label: "Privacy notice", href: "/legal/privacy" },
    sent: {
      title: "Enquiry received",
      text: "Thank you. Your enquiry is with the ELS team, and the reply will come to the email address you gave.",
      again: "Send another enquiry",
    },
  },
  next: {
    title: "What happens next",
    steps: [
      {
        name: "We read it",
        text: "A person on the ELS operations team reads every enquiry. There is no sales sequence.",
      },
      {
        name: "A short call",
        text: "We agree the task, the labels or rubric, the languages and what a good answer looks like.",
      },
      {
        name: "A scoped pilot",
        text: "You get a written scope: item count, overlap, gold rate, timeline and price, before any work starts.",
      },
      {
        name: "Results you can check",
        text: "Delivered labels come with gold accuracy and agreement figures, so you can judge the quality yourself.",
      },
    ],
  },
  contributors: {
    text: "Looking for paid work as a contributor? This form is for projects.",
    link: { label: "Apply as a contributor", href: "/contributors" },
  },
} as const;
