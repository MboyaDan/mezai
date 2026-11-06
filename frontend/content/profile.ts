// Choices on the contributor profile (brief 6.2). These are options a person
// can pick, not a statement of which languages MezAI covers (see Services).

export const languageOptions = [
  { value: "sw", label: "Kiswahili" },
  { value: "en", label: "English" },
  { value: "ki", label: "Gĩkũyũ" },
  { value: "luo", label: "Dholuo" },
  { value: "luy", label: "Luhya" },
  { value: "kam", label: "Kamba" },
  { value: "kln", label: "Kalenjin" },
  { value: "mas", label: "Maa (Maasai)" },
  { value: "so", label: "Somali" },
  { value: "am", label: "Amharic" },
  { value: "om", label: "Afaan Oromo" },
  { value: "lg", label: "Luganda" },
  { value: "rw", label: "Kinyarwanda" },
  { value: "yo", label: "Yorùbá" },
  { value: "ha", label: "Hausa" },
  { value: "ig", label: "Igbo" },
  { value: "zu", label: "isiZulu" },
  { value: "fr", label: "French" },
  { value: "ar", label: "Arabic" },
] as const;

export const proficiencyOptions = [
  { value: "native", label: "Native", hint: "First language, or used every day since childhood" },
  { value: "fluent", label: "Fluent", hint: "Read and write it easily, including nuance and slang" },
  { value: "working", label: "Working", hint: "Read and write it well enough for everyday text" },
] as const;

export const domainOptions = [
  { value: "health", label: "Health" },
  { value: "law", label: "Law" },
  { value: "finance", label: "Finance" },
  { value: "education", label: "Education" },
  { value: "software", label: "Software" },
  { value: "agriculture", label: "Agriculture" },
  { value: "other", label: "Other" },
] as const;

export const educationOptions = [
  { value: "secondary", label: "Secondary school" },
  { value: "certificate", label: "Certificate" },
  { value: "diploma", label: "Diploma" },
  { value: "bachelor", label: "Bachelor's degree" },
  { value: "master", label: "Master's degree" },
  { value: "doctorate", label: "Doctorate" },
  { value: "other", label: "Other" },
] as const;

/** Kenya first: the pilot pays through M-Pesa Kenya (A-038). */
export const countryOptions = [
  { value: "KE", label: "Kenya" },
  { value: "UG", label: "Uganda" },
  { value: "TZ", label: "Tanzania" },
  { value: "RW", label: "Rwanda" },
  { value: "ET", label: "Ethiopia" },
  { value: "SO", label: "Somalia" },
  { value: "NG", label: "Nigeria" },
  { value: "GH", label: "Ghana" },
  { value: "ZA", label: "South Africa" },
] as const;

const labels = new Map<string, string>(
  [...languageOptions, ...domainOptions, ...educationOptions, ...countryOptions].map((o) => [
    o.value,
    o.label,
  ]),
);

/** Human label for a stored code; unknown codes are shown as they are. */
export const labelFor = (code: string) => labels.get(code) ?? code;
