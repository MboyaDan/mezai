# Build plan, open questions and risks (Stage 0)

## One-page build plan

**Goal:** the pilot demo script (brief section 18) runs end to end on staging, with the public site passing the design gate and AC-01 to AC-11 green.

**Shape:** 21 stages (0 to 20), one PR each, merged by Dan. Stages that look too big for one sitting are split up front (marked ✂). I confirm a split before starting the stage.

| Stage | Outcome | Size | Split plan | Gate or AC |
|---|---|---|---|---|
| 0 | Direction, sitemap, wireframes, plan | S | | Your approval |
| 1 | Monorepo, API health, Compose, Next.js, CI | M | | CI green |
| 2 | Tokens, fonts, components, motion, three shells, `/design` | L | ✂ 2a tokens, fonts, primitives and shells · 2b data components, charts, motion | axe, 360 px |
| 3 | Schema, RLS, sqlc, seed | L | | RLS isolation test |
| 4 | Auth, permissions, audit, invitations | L | | AC-01, AC-02 |
| 5 | Home page storyboard | L | ✂ 5a frames 1 to 3, footer, SEO · 5b frames 4 to 8 | Design gate, Lighthouse |
| 6 | Other public pages, enquiry form, legal | L | ✂ 6a content pages · 6b contact, legal, errors | AC-03 |
| 7 | Contributor onboarding and qualification | M | | AC-04 |
| 8 | Projects, templates, import | M | | AC-05 |
| 9 | Assignment engine and task UI (3 types) | XL | ✂ 9a assignment service and API · 9b task workspace UI | AC-06, AC-07 |
| 10 | Quality engine | L | | AC-08 |
| 11 | Review, adjudication, deliverables | M | | AC-09 |
| 12 | Contributor dashboard | M | | Ledger reconciliation |
| 13 | Client and Ops dashboards | L | ✂ 13a client portal · 13b Ops overview | AC-10 |
| 14 | Payouts, manual M-Pesa path | M | | AC-11 (needs DG-03) |
| 15 | M-Pesa B2C | M | | DG-02, else skipped |
| 16 | Notifications | S | | |
| 17 | Security and privacy hardening | M | | `docs/security.md` |
| 18 | Performance, accessibility, design QA | M | | Real numbers |
| 19 | Staging deployment | M | | Demo script on staging |
| 20 | Pilot launch preparation | S | | DG-05 |

**Critical path to a first sales demo:** 1 → 2a → 5a/5b → 6 gives a credible public site early (about 7 PRs), while 3 → 4 → 7 → 8 → 9 → 10 → 11 builds the engine. I keep the stage order in CLAUDE.md; if you want the public site live sooner, Stage 19 (staging) could run a minimal frontend-only deploy right after Stage 6. Your call; default is the order as written.

**Working rules for usage limits:** every stage starts from `docs/PROGRESS.md`, reads only the files it needs, and ends by updating it. Large generated files (sqlc, OpenAPI types, lockfiles) are never printed into the conversation.

**Toolchain found on this machine (2 Nov 2026):** Go 1.25.7, Node 24.3.0, npm 11.4.2, Docker 29.3.1 with Compose v5.1.1, gh authenticated as MboyaDan. `pandoc` is **not** installed (see Q10).

## Open questions (grouped, with the default I will use)

Answer any you can; anything unanswered uses the default and is logged in `docs/ASSUMPTIONS.md`.

**A. Content and public site (brief section 20)**

| # | Question | Default |
|---|---|---|
| Q1 | Which languages can ELS staff confidently handle today? | English and Kiswahili on the public site; others "on request" |
| Q2 | Public contact email, phone and WhatsApp? | Email only, from `docs/content/`. **`docs/content/` is empty, so the email is hidden until you add one.** |
| Q6 | Kiswahili version of contributor pages? | English only, strings i18n-ready |
| Q8 | Photography: stock or a consented shoot? | Licensed stock with credits; layout ready for our own photos |

**B. Operations and money**

| # | Question | Default |
|---|---|---|
| Q3 | Default pay per task by task type? | Configurable per project; no public figures |
| Q4 | Should contributors see the client's name? | No, unless the project enables it |
| Q7 | Who is the second payout approver? | Required before Stage 14 goes live; a test user in development |
| Q9 | Retention after project close? | 90 days, configurable per project |

**C. Infrastructure**

| # | Question | Default |
|---|---|---|
| Q5 | Domain for staging and launch? | Vercel and Render default domains until DG-01 |
| Q10 | May I `brew install pandoc`? I read the brief this time by extracting the docx XML, which loses some table structure. | Keep extracting the XML; no install |
| Q11 | The GitHub repo is **public**, so the brief and design docs are visible to anyone. Make it private now? | Leave public until before real client data (brief section 11), as the brief allows |

**D. Conflicts between CLAUDE.md and the brief (the brief wins; please confirm)**

| # | Conflict | What I will do |
|---|---|---|
| C1 | CLAUDE.md Stage 4 lists six roles; the brief adds **Payout Approver** (`payout.approve`, cannot approve own batch). | Seven roles, including Payout Approver |
| C2 | CLAUDE.md shows agreement as three bands (strong, moderate, weak); the brief has four, adding **substantial**, and leaves 0.40 to 0.41 undefined. | Four bands: κ < 0.40 weak; 0.40 to 0.60 moderate; above 0.60 to 0.80 substantial; above 0.80 strong |
| C3 | CLAUDE.md Stage 3 lists a "content (pages …)" table; the brief says no CMS and content lives in typed files. | Only a `stats` table (value, source, verified_at) in the database; pages stay in `frontend/content/` |
| C4 | DESIGN.md frame 7 is titled "Security and how we work"; the brief calls it "How we handle data". | Use the brief's title on the page |

## Risks

Brief section 21 risks stand (overbuilding, credibility gap, data leakage, gold gaming, payout errors, regulatory change, name conflict, free-tier limits, usage limits). Additional risks found in Stage 0:

| Risk | Impact | Mitigation |
|---|---|---|
| `docs/content/` is empty | Public pages ship thin: no contact email, no language table, no About facts | Pages hide empty fields by design; I list exactly what content each page needs at the start of Stages 5 and 6 |
| No photography yet | Frame 4 and Contributors page look unfinished | Concept 1 lets diagrams carry the story; Frame 4 can ship with a lattice placeholder until Q8 is decided |
| GSAP plus SplitText and DrawSVG against the 170 KB home JS budget | Missed performance target | Lazy-load GSAP after first paint (budget excludes it), measure in Stage 5, cut network scrubbing on mobile if needed |
| Public repository | Brief, roadmap and later security notes are public | Q11; never commit secrets; make it private before real data |
| Status colours slightly below AA as text on ivory | Accessibility failures in workspace and portal | D-006 darkened values; Stage 2 axe checks |
| Scope of Stage 9 (three task types plus assignment engine) | Stage too big for one sitting | Split 9a and 9b |
| Kappa and trust formulas implemented subtly wrong | Misleading quality claims to clients | Textbook worked examples as unit tests (AC-08), percent agreement always shown alongside |
| Demonstration data mistaken for real data | Honesty breach | Every demo labelled "Demonstration"; sample values look obviously fake (`KES 00.00`) |
| DG-03 (M-Pesa CSV format) unresolved by Stage 14 | Payout export blocked | Build the export behind a column-mapping config; confirm format before Stage 14 |
