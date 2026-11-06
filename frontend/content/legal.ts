// Legal pages (brief 6.1 and 12). Drafts written from the brief and from what
// the software actually does. Not legal advice: a Kenyan advocate reviews them
// before launch (DG-05). Each document is versioned; Stage 7 stores the
// version a contributor accepted. Raise the version whenever the text changes.

import { brand } from "@/lib/brand";

export type LegalBlock = string | { list: string[] };
export type LegalSection = { heading: string; body: LegalBlock[] };

export type LegalDoc = {
  slug: string;
  title: string;
  /** Shown in the footer and page title. */
  short: string;
  version: string;
  /** ISO date of the last change. */
  updated: string;
  /** False until the advocate review in DG-05 approves this version. */
  approved: boolean;
  summary: string;
  sections: LegalSection[];
  /** Points the advocate must settle; shown on the page while it is a draft. */
  openPoints: string[];
};

const entity = brand.legalEntity;
const who = `${brand.name} is operated by ${entity}, ${brand.city}, Kenya`;
const contactLine =
  "To ask a question or exercise a right, write to us through the contact form on this site and say it is a data protection request.";

export const legalDocs: LegalDoc[] = [
  {
    slug: "privacy",
    title: "Privacy notice",
    short: "Privacy notice",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
    summary: `How ${entity} collects, uses and protects personal data on ${brand.name}, under Kenya's Data Protection Act, 2019.`,
    sections: [
      {
        heading: "Who we are",
        body: [
          `${who}. ${entity} decides why and how the personal data described here is used, so it is the data controller. When we process task data for a client, the client decides what that data is for and we act on their instructions.`,
        ],
      },
      {
        heading: "What we collect",
        body: [
          "We collect the minimum we need for each purpose.",
          {
            list: [
              "If you send a project enquiry: your name, organisation, email address and what you tell us about the project.",
              "If you apply as a contributor: your name, email address, a password (stored only as a one-way hash), languages and how well you know them, dialects, subject areas, education level, weekly availability and an M-Pesa phone number for payments.",
              "If you work as a contributor: the tasks you are assigned, your answers and their revisions, time spent on tasks, review decisions and feedback, gold-question results, your quality and trust scores, and your earnings and payouts.",
              "If you use the workspace for a client: your name, work email, organisation and role.",
              "For everyone who signs in: the date and version of the documents you accepted, your session, and a security record of important actions (who did what and when, with an IP address).",
            ],
          },
          "We do not ask contributors for identity documents during the pilot.",
        ],
      },
      {
        heading: "Why we use it",
        body: [
          {
            list: [
              "To reply to enquiries and plan projects (our legitimate interest in answering people who contact us).",
              "To run the contributor relationship: qualification, assigning tasks, measuring quality, paying for approved work (performing our agreement with you).",
              "To keep the platform secure: sign-in protection, rate limits and audit records (legitimate interest and legal obligation).",
              "To keep payment and tax records (legal obligation).",
              "Where we rely on your consent, we record which version you accepted and when, and you can withdraw it at any time.",
            ],
          },
        ],
      },
      {
        heading: "Quality scores and automatic decisions",
        body: [
          "Contributor work is scored against gold questions and compared with other contributors. If a trust score falls below the project's minimum, the account is paused automatically and the operations team is notified. You can ask for a person to review any automatic pause, and we will explain the scores behind it.",
        ],
      },
      {
        heading: "Who we share it with",
        body: [
          "We do not sell personal data.",
          {
            list: [
              "Clients see contributor ID numbers, skills and languages covered, and quality figures. They do not see contributors' names, email addresses or phone numbers.",
              "Contributors never see who submitted an enquiry, and see a client's name only if the project allows it.",
              "Safaricom processes M-Pesa phone numbers and amounts when we pay contributors.",
              "Service providers that host the platform, store files and send email process data on our instructions. They are listed in this notice before launch.",
              "Authorities, when the law requires it.",
            ],
          },
        ],
      },
      {
        heading: "How long we keep it",
        body: [
          {
            list: [
              "Client task content and submissions: deleted or anonymised 90 days after a project closes, sooner if the client asks.",
              "Payment, ledger and audit records: at least 7 years, for tax purposes. If you ask us to delete your data, we anonymise your identity in these records where the law allows.",
              "Enquiries that do not become a project: 24 months after the last contact.",
              "Contributor accounts: until you close the account, then as above for payment records.",
            ],
          },
        ],
      },
      {
        heading: "How we protect it",
        body: [
          "Contributors receive only the items assigned to them, one at a time, never a whole dataset. Each client organisation's data is separated in the database. Passwords are hashed, sessions expire, and phone numbers are masked in lists and logs.",
        ],
      },
      {
        heading: "Your rights",
        body: [
          "Under the Data Protection Act, 2019 you have the right to be told how your data is used, to see the data we hold about you, to object to its use, to have it corrected or deleted, and to receive it in a portable form.",
          contactLine,
          "If you are not satisfied with our answer, you can complain to the Office of the Data Protection Commissioner (ODPC).",
        ],
      },
      {
        heading: "Changes",
        body: [
          "When this notice changes, we raise its version number and date. Contributors are asked to accept a new version before their next task.",
        ],
      },
    ],
    openPoints: [
      "ODPC registration of ELS as data controller and processor (DG-05); add the registration details if required.",
      "Name the hosting, storage and email providers and where they process data (Stage 19), and the safeguards for any transfer outside Kenya.",
      "Confirm the lawful basis for each purpose and whether contributor processing needs consent as well as contract.",
      "Confirm the 24-month retention for enquiries that do not proceed (an assumption, A-032).",
      "Add a dedicated data protection contact address once one exists (Q2).",
    ],
  },
  {
    slug: "terms",
    title: "Terms of use",
    short: "Terms of use",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
    summary: `The terms for using the ${brand.name} website. Contributors also agree to the contributor terms; clients sign a separate agreement.`,
    sections: [
      {
        heading: "Who these terms are with",
        body: [`${who}. By using this website you agree to these terms.`],
      },
      {
        heading: "Using the site",
        body: [
          {
            list: [
              "Use the site lawfully and do not try to break, overload or get around its security.",
              "Do not scrape, copy in bulk or reuse the site's content for another service.",
              "Keep your sign-in details to yourself. You are responsible for what happens under your account.",
            ],
          },
        ],
      },
      {
        heading: "What the site says",
        body: [
          "We describe our services as accurately as we can. Figures on the site show their source and the date they were checked. Product screens labelled “Demonstration” use invented data. Nothing on the site is an offer; work starts only under a written agreement.",
        ],
      },
      {
        heading: "Ownership",
        body: [
          `The site's text, design and software belong to ${entity} or its licensors. Photographs are used under their photographers' licences and credited where they appear. Client data belongs to the client.`,
        ],
      },
      {
        heading: "Liability",
        body: [
          "The site is provided as it is. To the extent the law allows, we are not liable for losses caused by relying on the site's general information or by the site being unavailable. Nothing here limits liability that cannot be limited under Kenyan law.",
        ],
      },
      {
        heading: "Law",
        body: ["These terms are governed by the laws of Kenya, and Kenyan courts have jurisdiction."],
      },
    ],
    openPoints: [
      "Confirm the liability wording and governing-law clause.",
      "Decide whether MezAI is a registered business name of ELS (DG-01) and update the entity line.",
    ],
  },
  {
    slug: "contributor-terms",
    title: "Contributor terms",
    short: "Contributor terms",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
    summary: `The agreement between ${entity} and each person who does paid task work on ${brand.name}.`,
    sections: [
      {
        heading: "The relationship",
        body: [
          `You work as an independent contractor, not an employee of ${entity}. You choose when and how much you work. There is no minimum or guaranteed amount of work.`,
        ],
      },
      {
        heading: "Joining and qualifying",
        body: [
          "You must be at least 18 and give true details. You take a qualification test for each skill. We decide whether to approve an application and which projects you are eligible for.",
        ],
      },
      {
        heading: "Doing the work",
        body: [
          {
            list: [
              "Follow each project's instructions and rubric.",
              "Do the work yourself. Do not share your account or pass tasks to anyone else.",
              "Do not use AI tools to produce your answers unless the project instructions allow it.",
              "Skip a task, with a reason, if you cannot do it well. Report harmful content; you never have to complete a task you find distressing.",
            ],
          },
        ],
      },
      {
        heading: "Quality",
        body: [
          "Some tasks are gold questions with known answers, and some items are done by several contributors so answers can be compared. Your scores are shown to you with an explanation. Reviewers may accept or reject work; a rejection always comes with a reason you can see. If your trust score falls below a project's minimum, your account may be paused, and you can ask for a person to review it.",
        ],
      },
      {
        heading: "Pay",
        body: [
          "Each project states its rate per task before you start. You earn for each task that is approved. Approved earnings are paid to the M-Pesa number on your profile, in batches. Your dashboard shows pending, approved and paid amounts, and every payment carries its M-Pesa reference. If we make a mistake, we correct it with a new entry you can see; we never silently change past records.",
        ],
      },
      {
        heading: "Confidentiality",
        body: [
          "Task content belongs to our clients and is confidential. Do not copy, screenshot, download, share or paste it anywhere outside the platform, including into AI chatbots or translation tools, during or after your work. This obligation continues after you stop contributing.",
        ],
      },
      {
        heading: "Your work",
        body: [
          `Everything you submit for a task becomes the property of ${entity} or its client when it is paid for, and you give up any claim to it. You confirm the work is your own.`,
        ],
      },
      {
        heading: "Ending",
        body: [
          "You can stop at any time and ask us to close your account; approved earnings are still paid. We can pause or remove an account for breaking these terms or the acceptable use policy, and we tell you why.",
        ],
      },
    ],
    openPoints: [
      "Contractor classification, withholding tax and the emerging Kenyan fair-pay framework for AI data work (DG-04).",
      "Confirm the transfer of rights in submitted work and the confidentiality wording.",
      "Payout schedule and minimum payout amount, once Stage 14 sets them.",
      "Whether contributors outside Kenya can join, and what changes for them.",
    ],
  },
  {
    slug: "acceptable-use",
    title: "Acceptable use policy for contributors",
    short: "Acceptable use",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
    summary: "What contributors must and must not do on the platform. It is part of the contributor terms.",
    sections: [
      {
        heading: "Do",
        body: [
          {
            list: [
              "Answer honestly, from your own judgement, following the instructions.",
              "Take breaks. Skip tasks you cannot do well and say why.",
              "Report content that is harmful, illegal or should not be in the dataset.",
              "Write feedback and rationales that are clear and respectful.",
            ],
          },
        ],
      },
      {
        heading: "Do not",
        body: [
          {
            list: [
              "Share your account, hold more than one account, or let someone else work on your tasks.",
              "Copy, screenshot, download or share task content, or paste it into other tools or AI chatbots.",
              "Use AI tools or scripts to produce answers, unless the project instructions allow it.",
              "Try to find or guess gold questions, or coordinate answers with other contributors.",
              "Try to see items not assigned to you, or get around the platform's limits and security.",
              "Submit random or copied answers to earn faster.",
            ],
          },
        ],
      },
      {
        heading: "What happens if the policy is broken",
        body: [
          "Depending on how serious it is, we may reject the affected work, pause the account or remove it. We tell you the reason. Paid work that breaks this policy can be reversed with a visible correction in your ledger.",
        ],
      },
    ],
    openPoints: [
      "Confirm that reversing pay for work that breaks the policy is lawful and how it is notified.",
    ],
  },
  {
    slug: "cookies",
    title: "Cookie notice",
    short: "Cookies",
    version: "0.1",
    updated: "2026-11-06",
    approved: false,
    summary: `${brand.name} uses one cookie, only when you sign in. There are no analytics or advertising cookies.`,
    sections: [
      {
        heading: "The cookie we use",
        body: [
          "mezai_session: keeps you signed in. It is set only when you sign in, cannot be read by page scripts, is sent only over secure connections and to this site, and expires when you sign out, after 48 hours without activity, or after 7 days at most. It is strictly necessary, so it does not need consent.",
        ],
      },
      {
        heading: "What we do not use",
        body: [
          "No analytics, advertising, tracking or social media cookies, and no third-party cookies. Visitors who do not sign in get no cookies from us. Fonts and images are served from this site.",
        ],
      },
      {
        heading: "Changes",
        body: [
          "If we ever add a cookie that is not strictly necessary, we will update this notice first and ask for your consent before setting it.",
        ],
      },
    ],
    openPoints: [
      "If Cloudflare Turnstile is turned on for public forms (DG-07), describe what it processes before enabling it.",
    ],
  },
];

export const legalBySlug = (slug: string) => legalDocs.find((d) => d.slug === slug);

/** Formats an ISO date as "6 November 2026". */
export const legalDate = (iso: string) =>
  new Date(`${iso}T00:00:00Z`).toLocaleDateString("en-GB", {
    day: "numeric",
    month: "long",
    year: "numeric",
    timeZone: "UTC",
  });
