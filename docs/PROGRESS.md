# Progress

Read this first after `/clear`. Then read only what the next stage needs.

## Current state

- **Last stage:** 5c (people photo spread, follow-up to 5b). Branch `stage-05c-people-photos`, PR open. 5b merged as #19.
- **Next stage:** 6: Public website part 2. Planned split (D-011), confirm with Dan: 6a content pages (Services, Quality, Industries, Contributors, About, case studies index) and 6b Contact and enquiry form, legal pages, 404 and 500, AC-03 honesty test.
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
| Stats (API, fallback, filter) | `backend/internal/httpapi/stats.go`, `frontend/lib/stats.ts`, `frontend/content/stats.ts` |
| SEO files | `frontend/app/{robots,sitemap,opengraph-image}.ts(x)`, `frontend/lib/site.ts` |
| Home motion | CSS in `frontend/app/globals.css` (hero, services, swaps), GSAP in `frontend/components/home/home-motion.tsx` |
| Photo brief for Dan | `docs/content/PHOTO-BRIEF.md`, credits in `docs/content/image-credits.md` |
| Lighthouse record | `docs/screenshots/stage-05b/lighthouse.md` |
| API contract | `backend/openapi.yaml` |
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
| 5c | PR open | (see PR list) | Photo spread in frame 4 (D-053), photo pipeline with originals out of git (A-028), axe waits for animations. Commits pushed to the 5b branch after #19 merged, carried over unchanged. |

## Notes for the next session

- The brief can be read without pandoc: `unzip -p docs/brief.docx word/document.xml` and strip tags (see A-010).
- Planned splits: 2, 5, 6, 9, 13 (D-011). Confirm with Dan before starting each.
- `make up` then `make test` runs the PostgreSQL test; plain `go test` skips it unless `DATABASE_URL` is set (CI fails if unset).
- Frontend: Next.js 16.4 has breaking changes vs older versions; read `frontend/node_modules/next/dist/docs/` before using unfamiliar APIs (see `frontend/AGENTS.md`).
- Frontend dev dependencies have 17 moderate `npm audit` findings (minimatch ReDoS via ESLint); production deps have 0. Wait for upstream fixes via Dependabot.
- Components take semantic colours (`text-fg`, `bg-surface`, `border-border`...), never raw palette names, so they work on both themes. Put `data-theme="dark"` on dark bands.
- SignalLattice fills its container width; size it with a wrapper div.
- Use `m.*` from `motion/react`, never `motion.*` (LazyMotion strict throws). Section reveals: `Reveal`, `RevealGroup` + `RevealItem` from `@/motion/reveal`.
- New tables: grant `mezai_app` privileges in the same migration (no default privileges), add RLS if the table has `org_id` (a test enforces it), and revoke UPDATE/DELETE if append-only.
- Wrap SQL functions and DO blocks in `-- +goose StatementBegin/End`.
- New /v1 routes go through `api.handle(method, path, Access, handlers...)` in `router.go` with `Public()`, `SignedIn()` or `Perm("key", ...)` (any of), and must be added to `expectedAccess` in `authz_matrix_test.go`. Database work in handlers: `pool.InScope(ctx, PrincipalFrom(c).Scope(), ...)`, and write an `auth.Audit` entry in the same transaction for every change.
- Emails: `mail.Enqueue(ctx, q, to, template, data)` inside the transaction; add templates in `internal/mail/templates.go`.
- Public pages: add the route to `publicRoutes` in `frontend/lib/site.ts` (sitemap). Copy goes in `frontend/content/`, never inline claims.
- Public page motion: mark elements with `data-reveal`, `data-reveal-group`, `data-rule` or `data-connector`; `home-motion.tsx` does the rest. The test in `tests/e2e/home-motion.spec.ts` fails if anything stays hidden.
- Agreement is always number plus band: use `<Agreement>` or `agreementBand()` from `lib/agreement.ts`. The quality engine (Stage 10) computes kappa; the UI only displays it.
- Run Playwright against a production build with `ENABLE_DESIGN_PAGE=1 npm run build` then `CI=1 npx playwright test`.
