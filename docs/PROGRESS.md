# Progress

Read this first after `/clear`. Then read only what the next stage needs.

## Current state

- **Last stage:** 2a (Design system: tokens, fonts, components, shells). Branch `stage-02a-design-system`, PR open.
- **Next stage:** 2b: Table (sort, empty state), Pagination, Stat, ProgressBar, chart primitives (bar, line, sparkline, agreement matrix), Motion with shared variants and a `useReducedMotion` gate.
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
| API contract | `backend/openapi.yaml` |
| Decisions / assumptions / AC map | `docs/DECISIONS.md`, `docs/ASSUMPTIONS.md`, `docs/traceability.md` |

## Stage log

| Stage | Status | PR | Notes |
|---|---|---|---|
| 0 | Merged | #1 | No feature code. Brief read by extracting docx XML (pandoc missing). Concept tokens in `docs/design/concepts/tokens.css` are the starting point for Stage 2. |

| 1 | Merged | #2 | Go API (/healthz, /readyz), Compose (api, postgres 17, SeaweedFS, mailpit), Next.js 16 with /api rewrite and status page, CI, Dependabot. |
| 2a | PR open | (see PR list) | Tokens, fontsource fonts, Logo, SignalLattice, 25 components, three shells, `/design` with axe tests. |

## Notes for the next session

- The brief can be read without pandoc: `unzip -p docs/brief.docx word/document.xml` and strip tags (see A-010).
- Planned splits: 2, 5, 6, 9, 13 (D-011). Confirm with Dan before starting each.
- `make up` then `make test` runs the PostgreSQL test; plain `go test` skips it unless `DATABASE_URL` is set (CI fails if unset).
- Frontend: Next.js 16.4 has breaking changes vs older versions; read `frontend/node_modules/next/dist/docs/` before using unfamiliar APIs (see `frontend/AGENTS.md`).
- Frontend dev dependencies have 17 moderate `npm audit` findings (minimatch ReDoS via ESLint); production deps have 0. Wait for upstream fixes via Dependabot.
- Components take semantic colours (`text-fg`, `bg-surface`, `border-border`...), never raw palette names, so they work on both themes. Put `data-theme="dark"` on dark bands.
- SignalLattice fills its container width; size it with a wrapper div.
- Run Playwright against a production build with `ENABLE_DESIGN_PAGE=1 npm run build` then `CI=1 npx playwright test`.
