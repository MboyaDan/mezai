# Contributor screening funnel: research and proposal

Status: **approved (D-100)**, 2026-11-09. Built in Stage 7d (after 9b). Dan chose: adopt as proposed, form + optional CV, phone call with fixed questions, build after 9b.

## 1. The question

Dan wants a short, reusable way to confirm that a candidate can do the work before they reach paid projects:

- CV screening, then phone verification, then a short generalist assessment or interview.
- The interview or generalist check is done once and reused on every project.
- A project needing a different skill set asks only for a short skill assessment, never another interview.

## 2. What other platforms do

These are patterns only (CLAUDE.md rule 8). Most details come from third-party guides and forums, not the platforms' own documents; the sources at the end say which is which.

| Platform | Entry screen | Reuse across projects | Per-project step |
|---|---|---|---|
| DataAnnotation | A "Starter Assessment" chosen by track at sign-up (official FAQ), about an hour. Then a "Core" qualification of about 15 tasks (third-party). Identity check after the assessments. | Passing unlocks the platform; further specialty qualifications open more project types. | Specialty assessments per skill. Reported: no retake, no appeal. |
| Outlier (Scale) | Profile with CV/LinkedIn import, skills review, identity check, skill verification (official site). General onboarding 30 to 90 minutes. | The skills on the profile drive matching. | Short, unpaid, pass/fail project assessments with no feedback; failing one does not block others (third-party). |
| Mercor | A roughly 20-minute AI video interview, matched across all open roles; a second, tailored interview for specialised roles (press and official docs). | One interview counts across jobs (third-party; Mercor's docs describe retakes, not reuse). | A tailored interview for specialist roles. |
| Workada | A scheduled 15-minute interview, then weekly check-ins and a Slack community (Trustpilot reviews only; no official process found). | Not documented. | Not documented. |
| Toloka / Mindrift | Short qualification tasks unlock projects that match your expertise. | Toloka has system-wide "global skills" scored on the share of correct answers, plus skills each client defines (official docs). | Client-defined skills and golden sets. |
| CloudFactory (Kenya, Nepal) | A pre-assessment, then a practical, project-based assessment (Ruta N programme call); a domain test such as an annotation sample (aggregator). | Not documented. | Not documented. |

What they share:

1. **The cheapest filter goes first.** A profile or CV and an automated test come before anything that costs staff time.
2. **The entry check is general and done once.** It tests reading, reasoning, instruction-following and writing, not one project.
3. **Skill checks are short, per skill and reusable.** A new kind of project needs a new skill check only.
4. **Project onboarding is separate from qualifying.** It covers guidelines, a few calibration items, and the first tasks watched more closely.
5. **Recurring complaints:**
   - assessments that give no feedback;
   - "passed but no work";
   - long unpaid tests;
   - no retakes.

   MezAI can do better on all four, and the quality story we sell depends on it.

### What the research says about predicting performance

- **Structured interviews ranked first.** Sackett, Zhang, Berry and Lievens (2022) re-estimated the classic selection validities and put structured interviews top. They found every method's validity lower than earlier figures, by about .10 to .20, while the top-ranked methods mostly kept their rank.
- **Implication for the interview.** A short interview is worth keeping, but only if it is **structured**: the same questions and a fixed scoring rubric for every candidate.
- **Implication for the assessment.** A work sample (doing a few real tasks) is the closest thing to the job, and our qualification tests already work that way.

## 3. Recommendation for MezAI

Five steps. The first four happen once per person; the last happens once per skill.

```
1 Apply          sign-up, consent, profile + short experience form (optional CV)   [contributor, ~10 min]
2 Screen         Ops shortlists or declines with a reason                          [Ops, ~3 min each]
3 Core check     one 20–25 min generalist assessment, auto-scored + writing sample  [contributor, once]
4 Interview      10–15 min structured call to the M-Pesa number on the profile;     [Ops, once]
                 also confirms identity (D-078) and both working languages
   → active (contributor pool)
5 Skill check    10–15 min, 6–10 gold items per skill; reused on every project      [contributor, once per skill]
   → eligible for projects needing that skill (D-095)
   + project warm-up: first tasks of each new project at a higher gold rate, PAID, with feedback
```

### 3.1 Apply: a short experience form rather than a CV by default

- **Why not require a CV.** A CV is full of personal data we do not need: address, date of birth, photo, referees. The Data Protection Act asks us to collect the minimum, and a stored CV becomes a liability.
- **Instead, five short structured questions:**
  - highest relevant study or work;
  - experience with writing, translation, editing, teaching or a domain such as health or law;
  - any previous AI-data work;
  - hours available;
  - a 100-word "why this work".
- **CV and LinkedIn are optional.** A PDF of 2 MB or less, or a LinkedIn link, for people who want to add one.
- **Deleting CVs.** A CV is deleted 90 days after the screening decision, or at once if the person is declined. Only the decision and its reason are kept.
- **Legal note.** This needs privacy notice 0.3 before real candidates apply (DG-05).

### 3.2 Screen: Ops decides from a fixed checklist

- **Shortlist** or **decline**, with a reason the candidate sees.
- **Checklist:**
  - languages match current or planned projects;
  - writing in the form is clear;
  - availability;
  - no duplicate signals (7c).
- **Waiting list.** When there are no current projects in someone's languages, a third outcome, "waitlist", is kinder than a decline. It is also honest: it avoids the "passed but no work" complaint.

### 3.3 Core check: one generalist assessment, once per person

Built on the qualification-test engine that already exists. About 20 to 25 minutes:

| Part | Items | Scoring |
|---|---|---|
| Read instructions and apply them (short guideline, then classification items) | 6 | Auto (gold) |
| Compare two AI answers and say which is better and why | 2 | Auto (preference) + Ops reads the rationale |
| Spot the error (a factual or translation error planted in a short text) | 3 | Auto |
| Attention: an item whose instructions say to skip it | 1 | Auto |
| Short writing sample in English (and Swahili if they claim it): 80–120 words on a given prompt | 1 | Ops, 3-point rubric |

- **Pass mark.** 80% on the auto-scored part, and at least "adequate" on the writing sample.
- **One retake after 30 days.** The others' no-retake rule is a common complaint, and one cooling-off retake costs us little.
- **Feedback.** Candidates see their result and which part let them down. They never see the answers, so the gold stays usable.
- **Valid indefinitely.** It is never asked again. Trust scores (Stage 10) take over once the person works.

### 3.4 Interview: a structured 10–15 minute call, once per person

**Why a human call rather than an AI interview like Mercor's:**
- **Scale.** At 10 to 50 contributors in the pilot, staff time is small.
- **Data protection.** The Act (section 35) gives people the right not to be decided about solely by automated processing. A recorded video analysed by software also moves toward biometric data, which DG-05 has not cleared.
- **Bandwidth.** Many candidates will be on mobile data.

**Ops calls the M-Pesa number on the profile, which verifies the phone without an SMS provider (DG-06). The same questions every time:**

1. **Identity.** "Please confirm your full name and the last three digits of your ID." These must match the profile; this replaces a separate phone-verification step.
2. **Languages.** Two questions answered in Swahili and two in English (or the language claimed). Ops scores fluency 1 to 4.
3. **Scenario.** "You are halfway through a task and the instructions don't cover the case. What do you do?" Scored 1 to 4 against an anchored answer key (looks for: check the guidelines, skip with a reason, don't guess).
4. **Confidentiality.** "A friend asks you to show them a task you're working on. What do you say?" Pass or fail.
5. **Availability and device.** Hours a week, smartphone or computer, connectivity. Recorded, not scored.

**What is stored and decided:**
- **Stored:** the scores, a short note and the outcome (pass, fail, no answer after 3 tries). **No audio recording.**
- **Result:** a pass makes the person active, as long as the core check is passed and the identity verified (D-089).
- **Reuse:** the interview is never repeated for a new project.

### 3.5 Skill check: short, per skill, reused everywhere

This already exists: qualification tests per (kind, value), with D-095 matching. Rules to add:

- **Size:** 6 to 10 gold items, 10 to 15 minutes, pass mark 80%, one retake after 7 days (already A-035).
- **Reuse:** a skill held once counts on every project that needs it. Only a **new skill** asks for a new check, never a new interview.
- **Validity:** 12 months, or until the person's gold accuracy on that skill falls below the floor (Stage 10). Lapsed skills are re-checked with the same short test.

### 3.6 Project warm-up: paid, not another test

Each new project, including one in a skill the person already holds, starts with a **warm-up**:
- the first 5 tasks are served at a higher gold rate (for example 50%);
- the person sees whether each gold answer matched, with the project's explanation;
- the work is paid at the normal rate.

This gives the "project-specific onboarding" the other platforms have, without another unpaid test. It fits the fair-pay concerns under DG-04, and it feeds trust scores from day one.

**Unpaid time per candidate:** about 35 to 50 minutes in total:
- apply: about 10 minutes;
- core check: 20 to 25 minutes;
- interview: 10 to 15 minutes;
- plus 10 to 15 minutes per extra skill.

That is below the platforms' hour-plus starter tests, which is a selling point for recruiting.

## 4. How this fits what is built

| Piece | Today | Change |
|---|---|---|
| Sign-up, consent, profile | Stage 7 | Add the experience form and an optional CV (object storage exists; delete on schedule) |
| Screening | none | New `screening_status` (pending, shortlisted, waitlisted, declined) with reason; Ops queue |
| Core check | qualification tests (auto-scored, Ops approval) | A test of skill kind `core`; required before the interview; Ops reads the writing at approval (existing step) |
| Interview | none | New `interviews` record: rubric scores, note, outcome, interviewer; Ops screen; the identity confirmation feeds 7c verification |
| Phone verification | M-Pesa number entered twice; name check at payout (7c) | Interview call to that number = possession check; SMS one-time codes stay behind DG-06 |
| Activation | approved test + verified identity (D-089) | shortlisted + core passed + interview passed + identity verified |
| Skill checks | per skill, reused (D-095) | add 12-month validity |
| Warm-up | none | Stage 9 assignment: higher gold rate for a contributor's first N tasks in a project; feedback on those golds (Stage 10) |

**Suggested build:** "Stage 7d: screening funnel", about the size of 7c (API plus screens). It comes after 9b, and before any real candidate applies. Privacy notice 0.3 and contributor terms 0.3 go with it (CV handling, interview notes).

## 5. Decisions needed

1. Adopt this funnel (core check → interview → skill checks; warm-up instead of project tests)?
2. CV: optional with the experience form (recommended), or required?
3. Interview format: human structured phone call (recommended), or video call?
4. When to build 7d: after 9b (recommended), or before?

## Sources

- DataAnnotation FAQ (official): https://dataannotation.tech/faq
- DataAnnotation assessment guides (third-party): https://annotation.academy/blog/what-is-data-annotation-tech-assessment-like, https://aitrainer.work/guides/dataannotation-assessment-what-to-expect
- Outlier (official site): https://outlier.ai/ ; onboarding guides (third-party): https://annotation.academy/blog/how-to-get-started-on-outlier-after-acceptance, https://remotestack.in/blog/how-to-pass-outlier-ai-assessment-2026
- Mercor AI interview (official docs): https://talent.docs.mercor.com/support/ai-interview ; coverage: https://getcoai.com/news/how-mercor-uses-ai-to-conduct-interviews-and-match-candidates-with-jobs/
- Workada (reviews only): https://se.trustpilot.com/review/workada.com, https://remowork.life/companies/workada
- Toloka global skills (official docs): https://toloka.ai/docs/guide/nav-cross-project/ ; Mindrift: https://mindrift.ai/welcome
- CloudFactory assessments: https://rutanmedellin.org/en/offers/cloudfactory-challenge
- Sackett et al. (2022), "Revisiting meta-analytic estimates of validity in personnel selection", Journal of Applied Psychology 107(11): summary at https://www.siop.org/tip-article/is-cognitive-ability-the-best-predictor-of-job-performance and https://ink.library.smu.edu.sg/lkcsb_research/6894
