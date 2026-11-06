# Progress

Read this first after `/clear`. Then read only what the next stage needs.

## Current state

- **Last stage:** 7b (onboarding UI). Branch `stage-07b-onboarding-ui`, PR open. 7a merged as #23.
- **Next stage:** 8: Projects, templates and data import (organisations, project create, CSV/JSONL import with validation report, gold upload, publish/pause/close, client project requests; the gold page also gets "Make a qualification test", D-073). Confirm whether to split before starting.
- **Blocking questions:** none.

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
| Motion variants, gate, Reveal | `frontend/motion/` (provider in the root layout) |
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
| API types and client (frontend) | `frontend/lib/api/schema.ts` (generated: `npm run gen:api`), `lib/api/client.ts` |
| Session, sign-in gate, redirects | `frontend/lib/session.tsx`, `frontend/proxy.ts`, `lib/onboarding.ts` (`safeNext`) |
| Sign-in, join, verify, reset pages | `frontend/app/(auth)/`, `components/auth/` |
| Contributor portal (dashboard, profile, qualification) | `frontend/app/(contributor)/c/`, `components/contributor/`, answer inputs in `components/qualification/` |
| Workspace (overview, contributors, attempt review) | `frontend/app/(workspace)/w/`, `components/workspace/` |
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
| 7b | PR open | (see PR list) | Sign-in, join with consent, verify, reset; contributor dashboard steps, profile, qualification list and test (three task types, autosave); workspace overview, contributors, queue, attempt review, status changes; generated API types; full-stack AC-04 CI job. Fixed table overflow at 390 and robots rules. |

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
