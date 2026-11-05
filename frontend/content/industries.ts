// Industries page copy (brief 2.2 and 6.1): what each kind of buyer needs and
// which services fit. Words, not client logos; no client names or claims of
// past work.

import type { ServiceKey } from "@/content/home";

export type Industry = {
  id: string;
  name: string;
  needs: string;
  examples: string[];
  services: ServiceKey[];
};

export const industriesPage = {
  eyebrow: "Industries",
  title: { lead: "Different stakes.", accent: "Same", tail: "checks." },
  intro:
    "Teams come to us when a model has to work for people in African languages and African contexts, and they need evidence that the data behind it is right.",
  items: [
    {
      id: "ai-labs",
      name: "AI labs and model builders",
      needs:
        "Startups, research groups and model teams need evaluation and preference data that tests how a model handles African languages and everyday contexts such as law, health, agriculture and mobile money.",
      examples: [
        "Pairwise comparison of model responses against your rubric",
        "Evaluation sets in Kiswahili and code-switched text",
        "Expert review of a sample before you train on it",
      ],
      services: ["evaluation", "language", "review"],
    },
    {
      id: "telcos-banks",
      name: "Telcos, banks and insurers",
      needs:
        "Customer-facing chatbots and classifiers have to understand how customers actually write, often mixing Kiswahili and English in one message.",
      examples: [
        "Labelling customer messages by intent for routing",
        "Reviewing chatbot answers for accuracy and tone",
        "Translation review of help content and scripts",
      ],
      services: ["annotation", "evaluation", "language"],
    },
    {
      id: "health",
      name: "Health",
      needs:
        "A confident wrong answer about health can hurt someone. Health tools need reviewers who can tell safe guidance from plausible guidance, in the languages patients use.",
      examples: [
        "Rating model answers for medical safety with a clinical rubric",
        "Translation review of patient information",
        "Adjudication of disagreements by a domain reviewer",
      ],
      services: ["evaluation", "language", "review"],
    },
    {
      id: "education-research",
      name: "Education and research",
      needs:
        "Universities, researchers and education teams need documented, reproducible datasets: who labelled what, how often they agreed and how disagreements were settled.",
      examples: [
        "Annotated corpora with agreement reported per label",
        "Grant-funded dataset projects for African languages",
        "Evaluating tutoring or learning tools for clarity",
      ],
      services: ["annotation", "language", "evaluation"],
    },
    {
      id: "public-sector",
      name: "Public sector and NGOs",
      needs:
        "Services for citizens and communities have to reach people in their own language, and funders expect a clear record of how the data was produced and who was paid for it.",
      examples: [
        "Classifying feedback and reports from the public",
        "Translation review of public information",
        "A delivery manifest and payment records for your funder",
      ],
      services: ["annotation", "language", "review"],
    },
  ] satisfies Industry[],
  photo: {
    industry: "education-research",
    alt: "A teacher in a brown jacket writes on a whiteboard, holding notes in the other hand.",
  },
  cta: {
    title: "Not sure which service fits?",
    text: "Tell us what the model has to do and who it serves. We will suggest a small pilot.",
    primary: { label: "Start a pilot project", href: "/contact" },
  },
};
