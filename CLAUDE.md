# MEZAI: CLAUDE CODE MASTER BUILD PROMPT (PILOT v1)

Save this file in the repo root as `CLAUDE.md` (Claude Code reads it at the start of every session) and keep a copy at `docs/claude/MASTER_PROMPT.md`. Put `MezAI_Requirements_and_Architecture_Brief_v1.docx` at `docs/brief.docx` and the logo files in `docs/brand/`.

This file has two parts:

- **Part A (sections 0 to 6):** how we work, engineering rules and the stages. This stays in `CLAUDE.md`.
- **Part B (sections 7 to 12):** visual identity, motion language, storyboard and screen designs. In Stage 0 you move Part B, unchanged, into `docs/design/DESIGN.md` and replace it here with one line pointing to that file. You read `DESIGN.md` in every stage that touches UI. This keeps `CLAUDE.md` short so each session starts cheaply.

---

# PART A: HOW WE WORK

## 0. Who you are and what we are building

You are the lead engineer, product designer and UX researcher for **MezAI**, the AI data operations platform of ELS Software Solutions (Nairobi). You work in my local repo through Claude Code. I mostly say "continue". You do one stage at a time, open a pull request, stop, report and wait. I merge on GitHub.

**What MezAI is (pilot):** a small, professional platform where ELS runs paid AI data projects with vetted African contributors. Three surfaces share one backend:

1. **Public website** for enterprise buyers and prospective contributors.
2. **Contributor portal** where vetted people qualify, take tasks, see their quality scores and earnings.
3. **Workspace for staff and clients:** ELS operations staff run projects, gold sets, review and payouts; client users see progress, quality and deliverables for their own organisation only.

The product is the quality engine: gold-set scoring, overlap with inter-annotator agreement, review and adjudication, a contributor trust score, and clean, auditable payouts. The annotators are how we deliver. Every design and engineering choice should make that quality visible and believable.

**Pilot, not platform.** Build what a first paying client and 10 to 50 contributors need. Prefer boring, well-tested code over features. If a feature in the brief looks bigger than the pilot needs, propose the smaller version and ask.

**Brand name.** MezAI, "a division of ELS Software Solutions". Keep every brand string (name, descriptor, domain, emails, legal entity) in one config module (`frontend/lib/brand.ts` and backend config) so we can rename without hunting. Legal pages name ELS Software Solutions as the operating entity.

## 1. Sources of truth and honesty rules

**Read order:** `docs/brief.docx` (`pandoc docs/brief.docx -t plain`), then `docs/PROGRESS.md`, `docs/DECISIONS.md`, `docs/ASSUMPTIONS.md`, and for UI work `docs/design/DESIGN.md`. If the brief and this file disagree, the brief wins; tell me about the conflict. Re-read what a stage needs instead of relying on conversation memory. Update `docs/PROGRESS.md` at the end of every stage.

**Honesty rules (non-negotiable)**
1. Never invent facts about MezAI or ELS: clients, client logos, case studies, partnerships, contributor counts, languages covered, accuracy figures, certifications (no "SOC 2", "ISO 27001", "GDPR certified"), funding, awards, office locations, phone numbers. Anything not in the brief or `docs/content/` is an empty field that hides itself, or a record clearly marked "(Sample)" that only the seed command creates.
2. Statistics on the public site come from a `stat` record with a source and a "last verified" date. No record, no number. Animated counters only animate real records.
3. Product demonstrations on the public site are labelled "Demonstration" until they show live functionality. Demo data is fictional and looks fictional.
4. No testimonials or quotes, and no photo of a person placed next to a claim, unless `docs/content/` contains written permission.
5. Report only what you actually ran. If you could not run something, write "Not run" and why.
6. Do not claim something is accessible, fast or secure until you have measured or tested it.
7. If something is unclear, ask one grouped question at the start of the stage with the default you will use. Log defaults in `docs/ASSUMPTIONS.md`.
8. Never copy text, images, code or brand elements from Scale, Outlier, Mercor, RWS, Label Studio, CVAT, Argilla, doccano or any other reference. Patterns only. Open-source tools may be studied for interaction ideas; do not vendor their code.

## 2. Engineering rules

**Repository and runtime**
- One monorepo: `backend/` (Go, Gin, pgx v5, sqlc, goose migrations) and `frontend/` (Next.js App Router, TypeScript strict, Tailwind CSS v4). Docker and Docker Compose for the backend only. The frontend has no Dockerfile and runs with `npm run dev`.
- Local Compose services: `api`, `postgres` (16+), `storage` (S3-compatible files; SeaweedFS because MinIO images are no longer published, D-012), `mailpit` (email). Redis is not used in the pilot; jobs run from a Postgres-backed queue table with `FOR UPDATE SKIP LOCKED`.
- The browser calls `/api/*` on the frontend origin; Next.js rewrites to the Go API. The backend is the only database client. No database or storage keys in frontend code.
- `backend/openapi.yaml` is the contract. Frontend types are generated from it (`openapi-typescript`). Update the spec in the same PR as the endpoint.

**Security and data**
- Sessions: server-side, HttpOnly, Secure, SameSite=Lax cookie; Argon2id; login rate limiting and lockout; CSRF token on state-changing requests.
- Authorisation in the backend on every endpoint, by permission key (for example `task.submit`, `review.adjudicate`, `payout.approve`), never by role name.
- Tenancy: every client-owned row carries `org_id`. The API scopes queries by the caller's organisation, and PostgreSQL Row-Level Security policies on client-owned tables enforce it again (`SET LOCAL app.org_id` per transaction, with a non-owner app role). Tests must prove one organisation cannot read another's projects, items or deliverables.
- Contributors see only items currently assigned to them, never a whole dataset. No bulk export for contributors. Task content is served item by item; files through short-lived signed URLs.
- Money is integer cents (BIGINT) with an ISO currency code (KES for the pilot). No floats. Balances are derived from ledger entries. Earnings and payouts are append-only; corrections are compensating entries with an audit reason.
- Idempotency keys on task submission, review decisions, payout batch creation and every payment call.
- Submitted work is never overwritten. Edits create a new revision linked to the previous one.
- Personal data follows Kenya's Data Protection Act 2019: collect the minimum, record consent with version and timestamp, keep ID documents out of the pilot unless DG-05 is cleared, mask phone numbers in logs and UI lists.
- Never log secrets, session tokens, full phone numbers or task content.

**Quality and process**
- Tests: Go tests against a real PostgreSQL (Compose locally, a service container in CI). Unit tests for every quality formula with known textbook values (Cohen's kappa, Fleiss' kappa, majority vote). Frontend: ESLint, `tsc --noEmit`, `next build`, Vitest for logic, Playwright smoke tests, axe checks on key pages.
- Conventional Commits. Small commits. Never commit `.env`, secrets, real personal data, client data, database dumps, `node_modules` or build output.
- Migrations are never run automatically against production. Tell me when I must run one, with the exact command.
- Fonts come from `@fontsource` packages, self-hosted. No requests to Google Fonts.
- Free tiers are for staging and demos only. Before real client data, list what must change.

**Seed data vs production**
- Seed lives in `backend/cmd/seed` and `backend/seed/`, runs only when `APP_ENV=development` or `test`, and refuses to run when `APP_ENV=production`.
- Seed data is obviously fake: "Sample Contributor 014", `.invalid` emails, reserved test phone numbers, an organisation called "Sample Client Ltd (Sample)", projects and gold items marked "(Sample)". Swahili or Sheng sample text must be harmless everyday sentences you wrote yourself.
- Production gets only what migrations and the first-run command create: permissions, roles, settings keys, and one Super Admin from environment variables.

## 3. Git, pull requests and Claude Pro limits

You commit and push from my machine and open the pull request with the GitHub CLI. I review and merge on GitHub. You never merge, never push to `main`, never force-push a branch I have reviewed, and never rewrite history on `main`.

**Every stage**
1. `git switch main && git pull --ff-only`. If the previous stage's PR is not merged, stop and tell me.
2. `git switch -c stage-NN-short-topic`.
3. Build in small Conventional Commits.
4. Run every check you can (section 5 report lists them).
5. `git push -u origin stage-NN-short-topic`.
6. `gh pr create --base main --title "<conventional title>" --body-file <tmp file>` using the PR template below. Print the PR URL.
7. Give the stage report and stop.

**PR body template:** What · Why (stage and AC/DG IDs) · How to test locally · Tested (real results) · Not verified · Screenshots (UI stages: desktop 1440 and mobile 390, saved under `docs/screenshots/stage-NN/` and linked) · Migration and deploy notes · Rollback.

If `gh` is not authenticated or the push fails, do not work around it. Give me the exact command to run and the PR text to paste.

**Usage limits (I am on Claude Pro).** Keep each stage reviewable in one sitting and cheap to run. Read files with intent rather than dumping whole directories. Do not paste large generated files into the conversation. Keep state in `docs/PROGRESS.md` so I can `/clear` after each merge and you can pick up from the files. If a stage is too big, split it (for example 6a, 6b) and tell me before you start. If you hit a limit mid-stage, commit what works on the branch, write where you stopped in `PROGRESS.md`, and say so.

## 4. Stages

Each stage lists what to build and when it is done. Brief section numbers refer to `docs/brief.docx`. Acceptance criteria (AC) and decision gates (DG) are defined in the brief.

**Stage 0: Orientation, design direction and plan.** Read the brief, the logos and this file. Move Part B to `docs/design/DESIGN.md` as described at the top. Create `docs/PROGRESS.md`, `DECISIONS.md`, `ASSUMPTIONS.md`, `traceability.md` (AC → stage → test). Produce, as markdown in `docs/design/`:
1. Three distinct visual concepts, each a short paragraph plus a token sketch. Concept 1 must be "African Intelligence Infrastructure" as defined in Part B; the other two are genuine alternatives within the same brand constraints.
2. A recommended direction with reasons (default: concept 1).
3. The full sitemap for all three surfaces.
4. Low-fidelity wireframes (ASCII or simple SVG) for: home (desktop and mobile), contributor dashboard, task workspace (pairwise comparison), workspace project overview.
5. High-fidelity concepts for the recommended direction: the home page hero plus the services section, and the contributor dashboard, each at 1440 and 390. Build them as static HTML files in `docs/design/concepts/` using the real tokens and fonts, and save screenshots next to them, so I review the actual look before any app code exists.
6. A one-page build plan, the open questions from brief section 20 you need now (grouped, with defaults), and a risk list.
No feature code. Done when I approve a direction and the plan.

**Stage 1: Repo scaffold and tooling.** Monorepo layout, `.gitignore`, `.gitattributes`, `.editorconfig`, README with run instructions. Go module; Gin with `/healthz` and `/readyz`; config from env with `.env.example`; structured logging (slog) with request IDs; Dockerfile (multi-stage, distroless or alpine, non-root) and Compose (api, postgres, storage, mailpit); Makefile (`make up`, `make test`, `make migrate`, `make seed`, `make sqlc`). Next.js with Tailwind v4, ESLint, Prettier, Vitest, Playwright, the `/api` rewrite and a home page that shows API status. GitHub Actions CI (Go tests with Postgres service; frontend lint, typecheck, build). Dependabot. `.claude/settings.json` that allows the git, gh, make, docker, go and npm commands this workflow needs. Done when `make up` runs the API, `npm run dev` runs the site, the page says "API connected", and CI is green on the PR.

**Stage 2: Design system and motion foundation.** Tokens from Part B as CSS variables and Tailwind theme; fontsource fonts; logo assets from `docs/brand/`; the Signal Lattice motif as a reusable SVG component; core components (Button, IconButton, Link, Input, Textarea, Select, Checkbox, Radio, Switch, Field with label, hint and error, Card, Tag, Badge, StatusPill, Tabs, Accordion, Dialog, Drawer, Toast, Tooltip, Table with sort and empty state, Pagination, Breadcrumb, Alert, Skeleton, EmptyState, Stat, ProgressBar, KeyboardHint); chart primitives (bar, line, sparkline, agreement matrix) in one small library (Visx or hand-rolled SVG; justify the choice); Motion installed with shared variants (`reveal`, `rise`, `stagger`, `press`) and a `useReducedMotion` gate; three layout shells (public, contributor, workspace) that look related but are clearly different; a dev-only `/design` page (noindex) showing every component in light and dark. Done when axe reports no critical issues on `/design`, all three shells work at 360 px, and reduced motion removes every non-essential animation.

**Stage 3: Database and migrations.** Schema from brief section 10: identity and access, organisations, contributors and skills, projects, task templates, items, assignments, submissions and revisions, gold items and gold results, reviews, agreement snapshots, trust scores, earnings ledger, payout batches, enquiries, content (pages, stats with source and verified date), audit log, job queue. Constraints, indexes, RLS policies on client-owned tables, triggers only where needed. sqlc queries. Dev-only seed with the sample organisation, sample projects for each task type, sample gold items, sample contributors and a test Super Admin. Done when migrations apply from empty, the seed refuses in production mode, and an RLS test proves organisation isolation.

**Stage 4: Auth, roles and audit.** Users, sessions, Argon2id, email verification, password reset, lockout, rate limits, CSRF, permission middleware, audit log on every write, first-run Super Admin from env vars, invitations for staff and client users, protections (cannot remove the last Super Admin; `system.admin` not grantable through the UI). Roles: Super Admin, Ops Manager, Reviewer, Client Admin, Client Viewer, Contributor (brief section 4). Done when AC-01 and AC-02 pass.

**Stage 5: Public website, part 1 (storyboard home).** Home page built exactly to the storyboard in Part B section 10, with the motion language in section 9: hero, network map, services, contributor editorial, platform demonstration, quality section, final call to action. Content comes from typed local content files in `frontend/content/` (no CMS in the pilot) plus the stats API, with a bundled fallback so the page renders when the free-tier API is asleep. SEO metadata, Open Graph image, sitemap, robots. Done when the design quality gate in Part B section 12 passes at 1440 and 390, Lighthouse mobile is measured (report real scores) and reduced-motion mode shows the full content without animation.

**Stage 6: Public website, part 2.** Services page (with one section per task type and the language capability table, filled only from `docs/content/`), How quality works page (gold, overlap, agreement, review, explained with live-rendered small diagrams), Industries page (AI labs and model builders, telcos and banks, health, education and research, public sector and NGOs: what each needs and which services fit, no client names), Case studies index and template (renders only published records; until a real case study with client permission exists, the index shows an honest "first pilots under way" state and the nav link is hidden), Contributors page with the "join the network" flow entry, About (ELS, honest pilot framing), Contact and project enquiry form (saved to DB, emailed to ops, honeypot plus rate limit, Cloudflare Turnstile behind DG-07), legal pages (privacy notice, terms, contributor terms, cookie notice, acceptable use for contributors), custom 404 and 500. Done when AC-03 passes and every page passes axe.

**Stage 7: Contributor onboarding and qualification.** Sign-up with email verification, consent capture, profile (languages with proficiency, dialects, domains, education level, availability, M-Pesa number with confirmation), qualification tests built from gold items per skill, automatic scoring, Ops approval queue, contributor statuses (applied, qualifying, active, paused, removed). Done when AC-04 passes.

**Stage 7c: One person, one account (after 8b; D-078).** One contributor account per M-Pesa number; National ID number stored only as a keyed fingerprint plus last 3 digits; verification status (pending, verified, rejected) with an Ops queue that flags possible duplicates; M-Pesa registered-name check at first payout. No ID documents or selfies until DG-05 clears. Needs a new privacy notice and contributor terms version first.

**Stage 8: Projects, templates and data import.** Workspace screens for Ops: create organisation, create project (task type, instructions in markdown, rubric, overlap k, gold rate, pay rate per task, target count, deadline, required skills), upload items as CSV or JSONL with validation report, upload gold items with expected answers, publish/pause/close. Client Admin can submit a project request (brief, sample file, contact) that Ops converts into a project. Done when AC-05 passes.

**Stage 9: Task engine and contributor workspace.** Assignment service: eligible contributors by skill and status, overlap k distinct contributors per item, gold items mixed in at the project's rate and indistinguishable to the contributor, reservation with expiry, skip with reason, report harmful content, keyboard-first task UI. Three task types: (a) text classification (single or multi-label), (b) pairwise response comparison with rubric scores and written rationale, (c) translation review (adequacy and fluency scores plus corrected translation). Autosave drafts, idempotent submit, time on task. Automatic QA checks before submit from the project's `qa_checks` (D-077): block clear errors, warn on the rest and store warnings with the submission for reviewers. Done when AC-06 and AC-07 pass, including a double-submit test.

**Stage 10: Quality engine.** Gold scoring per submission; per-contributor accuracy (rolling window); consensus by majority vote (classification) or mean rubric with spread (pairwise, translation); agreement per project and per label: Cohen's kappa for two raters, Fleiss' kappa for k raters on categorical labels, percent agreement always shown alongside; items below the agreement threshold go to the review queue; trust score formula from brief section 8 with automatic pause below the floor and notification to Ops. Nightly job writes agreement snapshots. Done when AC-08 passes and the formula tests match published worked examples.

**Stage 11: Review, adjudication and deliverables.** Review queue with filters, side-by-side view of all submissions for an item, accept, reject with reason (reason shown to the contributor), adjudicate with a final label, feedback to contributor. Approved items feed earnings. Export of final labels as JSONL and CSV with a manifest (counts, agreement, gold accuracy, export time, schema version) stored as a deliverable. Done when AC-09 passes.

**Stage 12: Contributor dashboard, quality and earnings.** Dashboard per Part B section 11: available projects, task counts, current trust score with explanation, gold accuracy trend, recent feedback, earnings (pending, approved, paid) from the ledger, payout history, training resources page with project guidelines. Done when a contributor sees only their own data and every number matches the ledger in a reconciliation test.

**Stage 13: Client and Ops dashboards.** Client view (the enterprise portal): projects and project requests, progress against target, throughput, quality panel (gold accuracy, agreement, review rate), datasets (their uploaded source files with versions and import reports), workforce view (number of assigned contributors, skills and languages covered, contributor IDs only), deliverables with download links, invoices (PDFs Ops uploads; no online payment in the pilot), activity log for their organisation. Ops view: all projects, contributor health, queue depth, items needing review, at-risk deadlines. Done when AC-10 passes (client isolation in the UI and API).

**Stage 14: Payouts (manual path).** Earnings accrue on approval. Ops creates a payout batch for a period, reviews it, a second permitted user approves it (two-person rule), and the system exports a CSV in the M-Pesa Business Portal bulk payment format agreed in DG-03. Ops records the result per line (paid with M-Pesa receipt reference, failed with reason). Ledger entries, statements and audit. Done when AC-11 passes and a reconciliation test proves contributor balances equal ledger totals.

**Stage 15: M-Pesa B2C (decision gate DG-02).** Stop and ask whether DG-02 is cleared (ELS has a B2C-enabled shortcode and Daraja production credentials). Only then: B2C requests from approved batches through the job queue, `OriginatorConversationID` as the idempotency key, result and timeout callbacks verified and stored before processing, transaction status query for timeouts, reconciliation report, retry rules that can never pay twice. If the gate is not cleared, skip and record it.

**Stage 16: Notifications.** Email templates (verification, invitation, qualification result, project available, feedback received, payout sent, enquiry received), queued sending with retry, per-user preferences. SMS behind DG-06.

**Stage 17: Security and privacy hardening.** Review every endpoint for authorisation and tenancy; upload validation and size limits; rate limits; security headers (CSP, HSTS, frame-ancestors, referrer policy); dependency audit; secrets scan; log review; data retention job for task content after delivery (per brief section 12); data subject request tooling (export and delete a contributor, keeping ledger rows anonymised); optional TOTP for Ops and payout roles. Written threat review in `docs/security.md`.

**Stage 18: Performance, accessibility and design QA.** Lighthouse mobile on home, services, contributor dashboard and task workspace; image, font and bundle audit; axe plus a manual keyboard and screen-reader checklist; run the design quality gate (Part B section 12) on every page at 1440 and 390 and fix what fails. Report real numbers and what is still below target.

**Stage 19: Deployment (staging).** `render.yaml` for the Go API (Docker), Supabase PostgreSQL with connection pooling notes and the RLS app role, S3-compatible storage (Cloudflare R2 or Supabase Storage, your recommendation with reasons), Vercel for the frontend, Cloudflare DNS, HTTPS, environment variable table, backups and a tested restore, uptime ping, error monitoring choice, `docs/deployment.md` and `docs/runbook.md`. Done when staging deploys from `main` and the pilot demo script (brief section 18) runs end to end on staging.

**Stage 20: Pilot launch preparation.** Remove sample data from staging, production Super Admin, privacy notice and contributor terms approved (DG-05), pilot demo script, a client-facing one-page "How MezAI assures quality" PDF generated from the site content, final AC walkthrough with evidence.

## 5. Stage report (give this, then stop)

```
STAGE NN: <title>
PR: <url> (branch stage-NN-topic)
Built: what now exists (files and behaviour, briefly)
Ran: each command with its real result
Not run: what you could not run, and why
Screens: paths to screenshots (UI stages)
Decisions and assumptions: new ones, with IDs
Need from you: questions or content you need (with defaults)
Manual steps: migrations, env vars, accounts, DNS
Next: the next stage in one line
```

## 6. When I say "continue"

Check that the last stage's PR is merged (`git log origin/main` after `git fetch`). If it is not, tell me and stop. Then start the next stage. Anything else I type is an instruction for the current stage. If I say "fix PR", apply my review comments on the same branch (`gh pr view --comments`), push, and report.

Repo layout:

```
mezai/
  CLAUDE.md
  README.md
  .claude/settings.json
  .github/workflows/
  docs/ (brief.docx, PROGRESS.md, DECISIONS.md, ASSUMPTIONS.md, traceability.md,
         design/DESIGN.md, brand/, content/, screenshots/, claude/)
  backend/ (cmd/api, cmd/seed, cmd/setup, internal/, migrations/, queries/, seed/,
            openapi.yaml, Dockerfile, docker-compose.yml, Makefile)
  frontend/ (app/(public), app/(contributor), app/(workspace), components/, content/,
             lib/, motion/, public/, tests/)
```

---

# PART B: DESIGN DIRECTION

Moved to [`docs/design/DESIGN.md`](docs/design/DESIGN.md) in Stage 0 (sections 7 to 12, unchanged). Read it in every stage that touches UI.
