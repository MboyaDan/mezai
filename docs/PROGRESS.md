# Progress

Read this first after `/clear`. Then read only what the next stage needs.

## Current state

- **Last stage:** 11b (deliverables: exports with manifest, publishing, signed downloads). Branch `stage-11b-deliverables`, PR open. 11a merged as #39.
- **Next stage:** 12 (contributor dashboard, quality and earnings, DESIGN.md section 11): available projects, task counts, trust score with explanation, gold accuracy trend, recent feedback, earnings pending/approved/paid from the ledger, payout history, training page. Done when a contributor sees only their own data and every number matches the ledger in a reconciliation test.
- **Blocking questions:** none. Privacy notice 0.2 and contributor terms 0.2 are drafts awaiting the advocate (DG-05) before real contributors join.

## Where things are

| What | Where |
|---|---|
| Design direction (was CLAUDE.md Part B) | `docs/design/DESIGN.md` |
| Concepts and recommendation | `docs/design/concepts.md`, HTML and screenshots in `docs/design/concepts/` |
| Sitemap and routes | `docs/design/sitemap.md` |
| Wireframes | `docs/design/wireframes.md` |
| Plan, questions, risks | `docs/design/build-plan.md` |
| Research notes | `docs/design/research-notes.md` |
| Run instructions | `README.md` |
| Tokens | `frontend/app/globals.css` |
| Components and shells | `frontend/components/{ui,brand,shells}/`, gallery at `/design` (dev) |
| Charts | `frontend/components/charts/`, scale helper in `frontend/lib/charts/` |
| Motion variants, gate, Reveal | `frontend/motion/` (`MotionProvider` only in `/design` and screens that animate, e.g. `application-received.tsx`; D-050) |
| Schema and RLS | `backend/migrations/` (00007 has every policy and grant) |
| Queries and generated code | `backend/queries/*.sql` → `backend/internal/store/` (`make sqlc`) |
| Scoped transactions | `backend/internal/db/scope.go` (`db.InScope`, `db.Staff/Org/Contributor`) |
| Sample data | `backend/seed/` (run via `make seed`) |
| Auth (sessions, principal, audit, setup) | `backend/internal/auth/` |
| HTTP security (session, CSRF, access rules, rate limits) | `backend/internal/httpapi/security.go`, routes in `router.go` |
| Account flows (reset, verification, invitations, roles) | `backend/internal/auth/accounts.go`, handlers in `backend/internal/httpapi/accounts.go` |
| Email templates and outbox worker | `backend/internal/mail/` (mailpit UI at http://localhost:8025) |
| Home page sections and copy | `frontend/components/home/`, `frontend/content/home.ts` |
| Inner public pages | `frontend/app/(public)/{services,quality,industries,contributors,about,case-studies}/`, copy in `frontend/content/*.ts` |
| Enquiry endpoint (validation, honeypot, limits, Turnstile) | `backend/internal/httpapi/enquiries.go`; ops email template in `internal/mail/templates.go` |
| Contact page, form and client checks | `frontend/app/(public)/contact/`, `components/contact/`, `lib/enquiry.ts`, copy and choices in `content/contact.ts` |
| Legal documents (versioned drafts) | `frontend/content/legal.ts`, `components/legal/legal-document.tsx`, one route per document in `app/(public)/legal/` |
| 404 and 500 | `frontend/app/not-found.tsx`, `app/(public)/error.tsx`, `app/global-error.tsx`, `components/public/error-screen.tsx` |
| AC-03 honesty tests | `frontend/content/honesty.test.ts` (source), `tests/e2e/honesty.spec.ts` (rendered) |
| Shared public pieces (Container, PageHeader, CtaBand, IllustrativePhoto, PageMotion) | `frontend/components/public/` |
| Quality page live diagrams, kappa helper | `frontend/components/quality/diagrams.tsx`, `frontend/lib/kappa.ts` |
| Stats (API, fallback, filter) | `backend/internal/httpapi/stats.go`, `frontend/lib/stats.ts`, `frontend/content/stats.ts` |
| SEO files | `frontend/app/{robots,sitemap,opengraph-image}.ts(x)`, `frontend/lib/site.ts` |
| Page motion | CSS in `frontend/app/globals.css` (hero, services, swaps), GSAP in `frontend/components/public/page-motion.tsx` (home, services, quality) |
| Photo brief for Dan | `docs/content/PHOTO-BRIEF.md`, credits in `docs/content/image-credits.md` |
| Lighthouse record | `docs/screenshots/stage-05b/lighthouse.md` |
| API contract | `backend/openapi.yaml` (`TestOpenAPICoversRoutes` keeps it in step with the router) |
| Contributor onboarding (profile, consent, qualification, Ops queue, status machine) | `backend/internal/contributors/`, handlers in `backend/internal/httpapi/contributors.go`, sign-up in `backend/internal/auth/signup.go` |
| Answer checking (qualification now, gold in Stage 10) | `backend/internal/quality/answer.go` |
| Legal document versions the API accepts | `backend/internal/legal/legal.go` (kept equal to `frontend/content/legal.ts` by `content/legal.test.ts`) |
| Phone normalising and masking | `backend/internal/pii/phone.go` |
| Core check content and creation | `backend/internal/contributors/corecheck.go` |
| Screening funnel (application, CV, screening, interview) | `backend/internal/contributors/screening.go`; Ops panels `frontend/components/workspace/screening-panels.tsx`; rubric mirror `frontend/lib/screening.ts` |
| Task workspace and reports screens | `frontend/components/contributor/{work,task-workspace}.tsx`, helpers `frontend/lib/tasks.ts`, Ops `frontend/components/workspace/reports.tsx` |
| Background jobs (daily, SKIP LOCKED) | `backend/internal/jobs/`; quality jobs and reports in `backend/internal/tasks/reporting.go` |
| Review and adjudication (queue, decisions, reviewers, earnings) | `backend/internal/tasks/review.go`, `queries/review.sql`, handlers in `httpapi/tasks.go`; screens `frontend/components/workspace/review.tsx`, `projects/reviewers.tsx`, helpers `frontend/lib/review.ts` |
| Quality engine (gold scoring, consensus, trust, auto-pause) and maths | `backend/internal/tasks/quality.go`, `backend/internal/quality/agreement.go` |
| Task engine (eligibility, assignment, submit, reports) and QA checks | `backend/internal/tasks/`, handlers `backend/internal/httpapi/tasks.go`, checks `backend/internal/quality/qa.go` |
| National ID fingerprint, identity checks | `backend/internal/identity/`, `backend/internal/contributors/identity.go`; Ops panel `frontend/components/workspace/identity-panel.tsx` |
| API types and client (frontend) | `frontend/lib/api/schema.ts` (generated: `npm run gen:api`), `lib/api/client.ts` |
| Session, sign-in gate, redirects | `frontend/lib/session.tsx`, `frontend/proxy.ts`, `lib/onboarding.ts` (`safeNext`) |
| Sign-in, join, verify, reset pages | `frontend/app/(auth)/`, `components/auth/` |
| Contributor portal (dashboard, profile, qualification) | `frontend/app/(contributor)/c/`, `components/contributor/`, answer inputs in `components/qualification/` |
| Workspace (overview, contributors, attempt review) | `frontend/app/(workspace)/w/`, `components/workspace/` |
| Project screens (frame, tabs, form, imports, gold) | `frontend/components/workspace/projects/`, helpers in `frontend/lib/projects.ts`; requests and organisations in `components/workspace/{requests,organisations}.tsx` |
| Full-stack tests | `frontend/tests/full/` (`npm run test:full`, needs `make up migrate seed`) |
| Decisions / assumptions / AC map | `docs/DECISIONS.md`, `docs/ASSUMPTIONS.md`, `docs/traceability.md` |

## Stage log

| Stage | Status | PR | Notes |
|---|---|---|---|
| 0 | Merged | #1 | No feature code. Brief read by extracting docx XML (pandoc missing). Concept tokens in `docs/design/concepts/tokens.css` are the starting point for Stage 2. |

| 1 | Merged | #2 | Go API (/healthz, /readyz), Compose (api, postgres 17, SeaweedFS, mailpit), Next.js 16 with /api rewrite and status page, CI, Dependabot. |
| 2a | Merged | #10 | Tokens, fontsource fonts, Logo, SignalLattice, 25 components, three shells, `/design` with axe tests. |
| 2b | Merged | #14 | Table, Pagination, Stat, ProgressBar, Agreement, four charts, Motion variants and reduced-motion gate. Fixed a 2a overflow in Tabs at 360 px. |
| 3 | Merged | #15 | 7 migrations, 51 tables, RLS on 22, `mezai_app` role, sqlc store, Argon2id package, seed (2 sample orgs, 4 projects, 12 contributors), RLS and migration tests. |
| 4a | Merged | #16 | Login, logout, me; sessions (hashed tokens, idle and absolute expiry); CSRF; lockout; rate limits; route registry with access rules; audit writer; `make setup`. |
| 4b | Merged | #17 | Password reset, email verification, invitations (staff and client admin), user and role administration with Super Admin protections, email outbox worker, AC-01 matrix test, AC-02 complete. |
| 5a | Merged | #18 | Home frames 1 to 8 (static, reduced-motion form), public stats endpoint, cached stats with fallback, metadata, OG image, sitemap, robots. Fixed animation-delay under reduced motion. |
| 5b | Merged | #19 | CSS hero and hover motion, lazy GSAP scroll stories, demo autoplay with pause, Motion moved off public pages (JS 186 → 156 KB first load), Lighthouse mobile 93/100/96, photo brief. |
| 5c | Merged | #20 | Photo spread in frame 4 (D-053), photo pipeline with originals out of git (A-028), axe waits for animations. Commits pushed to the 5b branch after #19 merged, carried over unchanged. |
| 6a | Merged | #21 | Services (sticky index, language table per A-029), Quality (six steps with live gold, overlap, kappa and routing diagrams, trust score, security), Industries, Contributors, About, case studies index and template (D-056). Four new photo crops. |

| 6b | Merged | #22 | Contact page and `POST /v1/enquiries` (DB, ops email, audit, honeypot, limits, Turnstile off until DG-07), five versioned draft legal pages, 404/500, AC-03 honesty tests on source and rendered pages. Fixed: Vitest never ran `content/*.test.ts`; kappa e2e raced hydration. |
| 7a | Merged | #23 | Migration 00009 (`profile.manage`, test config, one open attempt per test, per-question results). 17 endpoints: sign-up with consent, profile, consents, qualification take/autosave/submit, Ops contributor list/detail/status, attempt queue/review/decision, tests from gold. Status machine, answer checking, phone masking, spec coverage test. |
| 7b | Merged | #24 | Sign-in, join with consent, verify, reset; contributor dashboard steps, profile, qualification list and test (three task types, autosave); workspace overview, contributors, queue, attempt review, status changes; generated API types; full-stack AC-04 CI job. Fixed table overflow at 390 and robots rules. |
| 8a | Merged | #25 | Migration 00010 (project QA checks, import digests, `project_requests` with RLS). 21 endpoints: organisations, task templates, projects (create, edit, lifecycle with readiness), CSV/JSONL import dry run and all-or-nothing commit, gold list/add/retire, client project requests with sample file, convert and decline. Object storage put/get. AC-05 API tests. Plan: auto QA checks (D-077, Stage 9) and one-account checks (D-078, new Stage 7c). |
| 8b | Merged | #26 | Projects list, create (and convert from a request), overview with readiness, status actions, settings with locks, item and gold import wizard (check → report → import), gold by hand, retire, "Make a qualification test" (D-073 done), organisations, client request form and Ops inbox with convert/decline. AC-05 full-stack UI test. |
| 7c | Merged | #27 | Migration 00011 (ID fingerprint + last 3 digits, identity status and M-Pesa name check on `contributor_profiles`, unique M-Pesa number and ID, `identity_flags`, `norm_name`; clears duplicate numbers left by test data). `internal/identity` (normalise, HMAC, key). `IDENTITY_HMAC_KEY` required outside dev/test. Profile takes `national_id`; reused numbers refused and flagged; profile saves rate limited per account. Ops verify/reject (`/contributors/{id}/identity`) and M-Pesa name check (`/mpesa-name-check`). Active needs approved test + verified identity. Privacy notice and contributor terms 0.2. UI: National ID field, dashboard identity step, Ops identity panel and "Identity checks" tab. Sign-out now does a full page load (fixes a redirect race). |
| 9a | Merged | #28 | Migration 00012 (`reported` assignment status, one reservation per contributor and project, `submissions.qa_warnings`). `internal/tasks`: eligibility, Next (overlap k, gold mixing, SKIP LOCKED), task view (content in contributor scope), drafts, idempotent submit with QA checks (`quality/qa.go`), skip, report, Ops report queue. 10 endpoints. AC-06 (1,000 assignments) and AC-07 tests. Seed qualifies active sample contributors. |
| 9b | Merged | #29 | Contributor Tasks list (`/c/work`), project brief (`/c/projects/[id]`), task workspace (`/c/work/[projectId]`) for all three task types: clock, autosave, keyboard, instructions drawer, skip/report dialogs, QA messages, idempotent submit. Ops content-report queue (`/w/reports`). Full-stack test creates its own projects through the API so reruns always have work. Screening funnel research and plan (D-100, Stage 7d). |
| 7d-1 | Merged | #30 | Migration 00013 (application answers, LinkedIn, CV columns, screening status, `interviews`). Application form and optional PDF CV (deleted on decline or by an hourly sweep 90 days after the decision); Ops shortlist / waitlist / decline; structured phone interview (fixed script and rubric, no recording); tests open only once shortlisted; activation = shortlisted + test + interview + verified identity. Privacy notice and contributor terms 0.3. Ops Applications and Interviews queues. |
| 7d-2 | Merged | #35 | Migration 00014 (`core` skill kind, one active core check, per-question `task_type`/`config`, `writing_rating`). Core check content in `corecheck.go` and `POST /qualification/core-check` (draft); writing answers (60–250 words) rated by Ops; core first, 30-day core retake; activation on the core check; skills valid 12 months. Screens: core check first on the tests list, mixed question types, writing input, Ops writing rating. |
| 9c | Merged | #36 | Concurrency test with 60 contributors found two races (items over k; same-key submits refused) and fixed them: recount under the item lock, lock the task before the key lookup. Own-only expiry plus a background sweep; 5 s lock and 30 s statement timeouts answered as 503 busy; per-account limits on task routes; pool of 20 connections. About 350 next-and-submit cycles a second locally. |
| 10a | Merged | #37 | `quality/agreement.go` (Cohen, Fleiss, percent, MAD, majority, Jaccard, spread, bands, trust formula) with textbook tests; brief 8.1 gold rules. Migration 00015 (`agrees_with_final`, one consensus per item, probation settings). On submit: gold scored, consensus or review routing, agreement per rater, trust recalculated, auto-pause below 0.70 with Ops email; probation gold rate. Profile shows the latest trust score. |
| 10b | Merged | #38 | `internal/jobs` runner on the Postgres `jobs` table (daily jobs per Nairobi date, SKIP LOCKED, backoff). Nightly agreement snapshots (percent, Cohen or Fleiss kappa per project and label, rubric MAD); daily gold-miss feedback that never names items; per-project warm-up gold rate; `GET /projects/{id}/quality` (aggregates, kappa bands) and Ops recompute; `/me/feedback`. Migration 00016 (warm-up settings, indexes). |
| 11a | Merged | #39 | Migration 00017 (`decision_source`, `review_id`, one review per item, `project_reviewers`; Stage 10 consensus items made final and paid). Consensus items become final and every answer earns; gold work accepted in the daily batch (D-110). Review queue and item API, decisions (accept consensus, choose, write final, reject item) with idempotency, per-answer rejection with reason shown to the contributor, feedback, agreement and trust; reviewer assignment per project (D-111, D-112). Screens `/w/review`, `/w/review/[id]`, reviewers panel; `/c/profile/complete` after the first complete profile (D-113). Fixed: gold feedback named the date and project in swapped places. |
| 11b | PR open | #40 | Migration 00018 (deliverable versions, size, creator; download file). `internal/delivery`: export of final regular items as JSONL and CSV with the brief 8.5 manifest in object storage, publish per version, five-minute links signed per user, checksum check and a `deliverable_downloads` row plus audit on every download (D-115, D-116). Export schema per task type with CSV formula guard (D-117). Deliverables tab for Ops and clients. AC-09 done. |

## Notes for the next session

- The brief can be read without pandoc: `unzip -p docs/brief.docx word/document.xml` and strip tags (see A-010).
- Planned splits: 2, 5, 6, 9, 13 (D-011). Confirm with Dan before starting each.
- `make up` then `make test` runs the PostgreSQL test; plain `go test` skips it unless `DATABASE_URL` is set (CI fails if unset).
- Frontend: Next.js 16.4 has breaking changes vs older versions; read `frontend/node_modules/next/dist/docs/` before using unfamiliar APIs (see `frontend/AGENTS.md`).
- Frontend dev dependencies have 17 moderate `npm audit` findings (minimatch ReDoS via ESLint); production deps have 0. Wait for upstream fixes via Dependabot.
- Components take semantic colours (`text-fg`, `bg-surface`, `border-border`...), never raw palette names, so they work on both themes. Put `data-theme="dark"` on dark bands.
- SignalLattice fills its container width; size it with a wrapper div.
- Use `m.*` from `motion/react`, never `motion.*` (LazyMotion strict throws). Section reveals: `Reveal`, `RevealGroup` + `RevealItem` from `@/motion/reveal`.
- Local databases seeded before 7a have empty qualification test `config`: re-seed a fresh volume, or run the UPDATE in the 7b PR notes.
- Done in 9b: task screen uses `/v1/me/projects`, `/next`, `/me/tasks/{id}` (+ `/draft`, `/submit` with `Idempotency-Key`, `/skip`, `/report`); QA settings come with the project (`qa_checks`, `min_rationale_chars`) for client-side hints; `qa_blocked` answers list `issues`. Ops content-report screen also belongs to 9b (API: `/v1/content-reports`).
- Local databases seeded before 7c/9a: run the fix-up SQL in the 9b PR notes (verified identities, sample qualifications), or re-seed a fresh volume. Contributors who accepted 0.1 legal documents see the 0.2 consent prompt first.
- Stage 10: gold scoring reads gold-ness from `gold_items.item_id`; submissions on gold items stay out of consensus. Stage 14: require `mpesa_name_checked_at` before a contributor's first payout (D-092). Stage 17: decide whether deletion keeps the ID fingerprint (A-053).
- Sign-up allows 10 an hour per IP: repeated full-stack runs hit it; `docker compose restart api` resets it locally.
- Contributor endpoints call `contributorOf(p)`: Super Admin holds every permission but has no contributor profile.
- Qualification answers use the task templates' submission shapes; reuse `quality.Parse` and `quality.Matches` for gold in Stages 9 and 10.
- New tables: grant `mezai_app` privileges in the same migration (no default privileges), add RLS if the table has `org_id` (a test enforces it), and revoke UPDATE/DELETE if append-only.
- Wrap SQL functions and DO blocks in `-- +goose StatementBegin/End`.
- New /v1 routes go through `api.handle(method, path, Access, handlers...)` in `router.go` with `Public()`, `SignedIn()` or `Perm("key", ...)` (any of), and must be added to `expectedAccess` in `authz_matrix_test.go`. Database work in handlers: `pool.InScope(ctx, PrincipalFrom(c).Scope(), ...)`, and write an `auth.Audit` entry in the same transaction for every change.
- Emails: `mail.Enqueue(ctx, q, to, template, data)` inside the transaction; add templates in `internal/mail/templates.go`.
- Public pages: add the route to `publicRoutes` in `frontend/lib/site.ts` (sitemap). Copy goes in `frontend/content/`, never inline claims.
- Public page motion: mark elements with `data-reveal`, `data-reveal-group`, `data-rule` or `data-connector` and render `<PageMotion />` (home, services, quality only). `tests/e2e/home-motion.spec.ts` fails if anything stays hidden.
- Inner public pages: `PageHeader` (one h1), sections, `CtaBand`. Stock photos go through `IllustrativePhoto` and get a row in `docs/content/image-credits.md`; crops in `frontend/scripts/prepare-photos.mjs`.
- New public pages: add to `pages` in `tests/e2e/pages.spec.ts` (axe, overflow, sourced stats).
- Agreement is always number plus band: use `<Agreement>` or `agreementBand()` from `lib/agreement.ts`. The quality engine (Stage 10) computes kappa; the UI only displays it.
- Run Playwright against a production build with `ENABLE_DESIGN_PAGE=1 npm run build` then `CI=1 npx playwright test`.
- Legal versions: `legalBySlug("privacy").version` etc. Stage 7 stores the accepted version with the consent; raise `version` and `updated` whenever the text changes, and set `approved: true` only after DG-05.
- Demonstrations and worked examples carry `data-demo`; the honesty tests require a visible "Demonstration" or "Worked example" label in each.
- New public pages: also covered automatically by `honesty.spec.ts` if added to `publicRoutes` in `lib/site.ts`.
- Docker Desktop did not start during 6b; Go tests against PostgreSQL ran only in CI for that stage.
- Projects: `internal/projects` (service) and `internal/imports` (pure file checks). Task-type rules (config, item fields, gold answers, QA checks) live in `internal/quality/content.go`; add a task type there first.
- Uploads go through `storage.Blobs` (`AuthDeps.Files`); tests use `storage.NewMemory()` via `newEnv`, and `e.files.Corrupt` simulates a changed file. `S3_TEST_ENDPOINT=http://localhost:8333 make test` also round-trips real SeaweedFS.
- Multipart endpoints in tests: `e.multipart(path, fields, fileField, fileName, content, session)`; `e.upload(...)` for imports.
- Project pages read the project with `useProject()` (from `ProjectFrame` in `/w/projects/[id]/layout.tsx`); call `reload()` after changes that affect counts.
- Next 16 keeps the previous route's DOM hidden: in Playwright use `getByRole`, or add `.filter({ visible: true })` to `getByText` / `getByLabel`.
- Local full-stack reruns hit the login limits (20/min per IP, 10 per 15 min per email). `docker compose restart api` resets them (in memory).
- `npm run dev` prints "Could not validate instant navigation" for client-loaded workspace routes (also `/w` from 7b). Dev-only; `npm run build` passes and the full-stack tests run against it.
- Earnings: `AccrueEarning` (one per accepted submission). Contributors are paid for every accepted answer (D-110); Stage 12 shows pending (submitted, not yet decided) vs approved (ledger accruals) vs paid.
- Review radio groups need per-instance names (`useId`): Next keeps the previous item mounted while hidden.
- Deliverables: `internal/delivery` (service, files, links). Add a task type's CSV columns in `quality.ExportColumns`/`ExportValues`; raise `quality.ExportSchemaVersion` when a field changes meaning. The deliverables list offers the manifest through the JSONL row (`file: manifest`).
