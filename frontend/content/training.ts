// Contributor training page copy (Stage 12b, brief 6.2: project guidelines,
// worked examples, FAQs). Examples are written for MezAI, fictional and
// labelled "Worked example". Numbers that are settings (weights, floor,
// minimum payout) are not written here; the page reads them from the API.

export type WorkedExample =
  | {
      id: string;
      taskType: "classification";
      title: string;
      instruction: string;
      text: string;
      labels: string[];
      answer: string;
      why: string;
    }
  | {
      id: string;
      taskType: "pairwise";
      title: string;
      instruction: string;
      prompt: string;
      a: string;
      b: string;
      answer: "A" | "B";
      rationale: string;
      why: string;
    }
  | {
      id: string;
      taskType: "translation";
      title: string;
      instruction: string;
      source: string;
      machine: string;
      adequacy: number;
      fluency: number;
      corrected: string;
      why: string;
    };

export const workedExamples: WorkedExample[] = [
  {
    id: "classification",
    taskType: "classification",
    title: "Text classification",
    instruction: "Choose the team that should answer this customer message.",
    text: "Nimelipa bili yangu mara mbili mwezi huu. Naomba mnirudishie pesa.",
    labels: ["Billing", "Network", "Account", "Other"],
    answer: "Billing",
    why: "The customer paid a bill twice and asks for a refund. That is a billing question, even though it also mentions money coming back to their account.",
  },
  {
    id: "pairwise",
    taskType: "pairwise",
    title: "Pairwise comparison",
    instruction: "Which response answers the question more helpfully and accurately?",
    prompt: "How long should I boil drinking water to make it safe?",
    a: "Bring the water to a rolling boil and keep it boiling for at least one minute, then let it cool covered.",
    b: "Water is safe once it is warm. Boiling is only needed if it looks dirty.",
    answer: "A",
    rationale:
      "A gives a clear, safe instruction. B is wrong: warm water is not safe, and clear-looking water can still carry germs.",
    why: "A good rationale names what makes the chosen response better and what is wrong with the other one, in your own words.",
  },
  {
    id: "translation",
    taskType: "translation",
    title: "Translation review",
    instruction: "Score the machine translation from English to Swahili, then correct it.",
    source: "I paid my bill twice this month.",
    machine: "Nimelipa bili yangu mara moja mwezi huu.",
    adequacy: 2,
    fluency: 5,
    corrected: "Nimelipa bili yangu mara mbili mwezi huu.",
    why: "The Swahili reads naturally (fluency 5), but “mara moja” means “once”, which changes the meaning (adequacy 2). The correction changes only what was wrong.",
  },
];

export const faqs: { id: string; q: string; a: string }[] = [
  {
    id: "paid",
    q: "When do I get paid?",
    a: "Each accepted task adds the project's rate to your approved balance. Approved balances are paid to your M-Pesa number in batches once they reach the minimum shown on your Earnings page. Every payment shows its M-Pesa receipt on your statement.",
  },
  {
    id: "pending",
    q: "Why is some of my work pending?",
    a: "A task is pending until it is decided: when enough people have answered the same item, or when a reviewer has looked at it. The pending amount is an estimate at the project's current rate.",
  },
  {
    id: "checks",
    q: "What are checks with known answers?",
    a: "Some tasks have an answer the MezAI team already agreed on. They look exactly like other tasks, so treat every task the same. Matching them is your gold accuracy. They are paid like any other task.",
  },
  {
    id: "daily",
    q: "Why did my trust score not change after a task?",
    a: "Your score and trend are shown as they stood at the start of each day. Today's work shows tomorrow, together with the daily note about any checks you missed.",
  },
  {
    id: "floor",
    q: "What happens if my score drops below the floor?",
    a: "New tasks pause and the MezAI team looks at your recent work. Your earnings and record stay visible, and approved work is still paid.",
  },
  {
    id: "not-accepted",
    q: "Why was a task not accepted?",
    a: "A reviewer can mark a single answer as not accepted when it was careless or ignored the instructions. That answer earns nothing, and the reviewer's reason appears in your feedback. Disagreeing with others in good faith is not a reason to refuse payment.",
  },
  {
    id: "skip",
    q: "Can I skip a task?",
    a: "Yes. Skip any task you cannot do well, with a short reason. Report anything harmful; it leaves your queue at once. You never have to finish a task that distresses you.",
  },
  {
    id: "private",
    q: "Can I share or copy task content?",
    a: "No. Task content belongs to the client and is confidential. Do not copy it, screenshot it or paste it into other tools.",
  },
];
