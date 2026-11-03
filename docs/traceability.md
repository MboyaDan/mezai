# Traceability: acceptance criteria → stage → test

Test names are planned; each stage replaces "planned" with the real test path and its last result.

| AC | Criterion (short) | Stage | Test (planned) | Status |
|---|---|---|---|---|
| AC-01 | Every endpoint checks a permission key; role × endpoint matrix | 4 (re-run 17) | 4a: route registry, `TestEveryV1RouteDeclaresAccess` in `backend/internal/httpapi/auth_test.go` (done). 4b: role × endpoint matrix test | Partly done |
| AC-02 | Lockout, reset, CSRF, audit; last Super Admin protected | 4 | 4a: lockout, CSRF, audit in `backend/internal/httpapi/auth_test.go` (done). 4b: password reset, last Super Admin | Partly done |
| AC-03 | No stat without a source; no logos, testimonials, certifications; demos labelled | 6 (started 5) | `frontend/tests/honesty.test.ts` (fails build on a sourceless stat) | Planned |
| AC-04 | Sign-up, consent (version and time), profile, qualification, approval | 7 | `backend/internal/contributor/onboarding_test.go`, Playwright `join.spec.ts` | Planned |
| AC-05 | Project creation; CSV/JSONL import with row errors; invalid file imports nothing | 8 | `backend/internal/importer/importer_test.go` | Planned |
| AC-06 | Never the same item twice; exactly k contributors; gold rate within tolerance over 1,000; gold indistinguishable | 9 | `backend/internal/assign/assign_test.go` | Planned |
| AC-07 | Same idempotency key twice → one submission, one earning | 9 | `backend/internal/task/submit_idempotency_test.go` | Planned |
| AC-08 | Gold accuracy, Cohen's κ, Fleiss' κ, consensus, trust match worked examples; auto-pause below floor | 10 | `backend/internal/quality/*_test.go` | Planned |
| AC-09 | Routing to review; adjudicated items final and immutable; correct manifest | 11 | `backend/internal/review/*_test.go`, `export/manifest_test.go` | Planned |
| AC-10 | Client cannot read another org's data via UI, API or DB app role | 3 (RLS), 13 | `backend/internal/db/rls_test.go` (DB layer: done in Stage 3), Playwright `client-isolation.spec.ts` | Partly done (DB) |
| AC-11 | Creator cannot approve batch; export totals = approved earnings; balances = ledger | 14 | `backend/internal/payout/*_test.go` | Planned |

Other "done when" checks without an AC ID:

| Check | Stage | Test (planned) |
|---|---|---|
| Seed refuses in production | 3 | `backend/cmd/seed/main_test.go` (done) |
| Migrations apply from empty, roll back to empty, apply again | 3 | `backend/migrations/migrations_test.go` (done) |
| Every table with `org_id` has RLS (allowlist for staff-only tables) | 3 | `backend/migrations/migrations_test.go` (done) |
| Contributors see only items they currently hold; never gold rows | 3 (DB), 9 (API) | `backend/internal/db/rls_test.go` (DB done) |
| Append-only tables reject update and delete from the app role | 3 | `backend/internal/db/rls_test.go` (done) |
| Reconciliation: dashboard numbers equal ledger | 12 | `backend/internal/ledger/reconcile_test.go` |
| axe: no critical issues on `/design` and key pages | 2, 6, 18 | Playwright plus axe |
| Reduced motion removes non-essential animation | 2, 5 | Playwright with `reducedMotion: 'reduce'` |
| Design quality gate (DESIGN.md section 12) | every UI stage | Stage report answers |
