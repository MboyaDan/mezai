# Traceability: acceptance criteria → stage → test

Test names are planned; each stage replaces "planned" with the real test path and its last result.

| AC | Criterion (short) | Stage | Test (planned) | Status |
|---|---|---|---|---|
| AC-01 | Every endpoint checks a permission key; role × endpoint matrix | 4 (re-run 17) | `TestAuthorisationMatrix` in `backend/internal/httpapi/authz_matrix_test.go` (every role × every guarded route, against tables from brief section 4) and `TestEveryV1RouteDeclaresAccess` | Done (re-run as routes are added) |
| AC-02 | Lockout, reset, CSRF, audit; last Super Admin protected | 4 | `backend/internal/httpapi/auth_test.go` (lockout, CSRF, audit), `accounts_test.go` (`TestPasswordReset`, `TestSuperAdminProtections`) | Done |
| AC-03 | No stat without a source; no logos, testimonials, certifications; demos labelled | 6 (started 5) | 5a: `frontend/lib/stats.test.ts` (sourceless stats dropped), `backend/internal/httpapi/stats_test.go` (only published, sourced stats served), `smoke.spec.ts` (every rendered stat names its source; demo labelled). 6a: `content/case-studies.test.ts` (unpublished, unpermitted or unsourced case studies never render), `pages.spec.ts` (every page: stats name a source, demonstrations and worked examples labelled, unpublished case-study slugs 404, no nav link). 6b: `content/honesty.test.ts` (source scan for certification, logo, testimonial, award, funding and headcount claims; no quotation markup; fallback stats publishable; every `data-demo` labelled), `tests/e2e/honesty.spec.ts` (every public route and the 404: same claims, sourced stats, labelled demos, illustrative photo captions). Note: `content/*.test.ts` only started running in 6b (Vitest include fixed) | Done |
| AC-04 | Sign-up, consent (version and time), profile, qualification, approval | 7 | 7a (API): `TestContributorOnboarding` in `backend/internal/httpapi/onboarding_test.go` (sign-up with consent version and time stored, verification, profile with double-entered M-Pesa number, test taken and scored, idempotent submit, Ops approval making the contributor active, rejection note and retake cooldown), `TestContributorStatusByOps`, `TestQualificationTestFromGold`, `TestConsentAcceptance`; `contributors/status_test.go` (every transition), `quality/answer_test.go`, `pii/phone_test.go`, `frontend/content/legal.test.ts`. 7b: `frontend/tests/full/join.spec.ts` (CI job "Full stack (AC-04)": sign-up with consent, mailpit link, profile, test, Ops approval, active; axe on 7 screens), `lib/onboarding.test.ts` | Done (passed locally; CI on PR) |
| AC-05 | Project creation; CSV/JSONL import with row errors; invalid file imports nothing | 8 | 8a (API): `TestProjectImportAC05` in `backend/internal/httpapi/projects_test.go` (create project, row-level report, invalid file and second commit import nothing, reused IDs refused, gold import, publish rules, changed file and stale file refused), `TestImportPermissionsAndFiles`, `TestClientProjectIsolation`, `TestProjectRequests`; `internal/imports/imports_test.go`, `internal/quality/content_test.go`, `internal/storage/storage_test.go`. 8b: `frontend/tests/full/projects.spec.ts` (client request with sample, Ops convert, file with a bad row imports nothing, valid file imports all rows, gold by hand, qualification test from gold, ready/publish rules, settings lock, client view without staff fields, other organisation gets not found; axe on 8 screens, no sideways scroll at 390); `frontend/lib/projects.test.ts` | Done |
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
| Pre-submit QA checks: blocking checks stop a submission, warnings are stored with it (D-077) | 8 (settings), 9 (enforced) | 8a: `quality/content_test.go` (`TestQAChecks`). 9: submit endpoint and task screen tests |
| One contributor account per person: unique M-Pesa number and ID fingerprint, duplicates flagged for Ops (D-078) | 7c | Planned |
| Clients never see gold files or another organisation's imports and requests | 8 | `TestClientProjectIsolation`, `TestProjectRequests` (done) |
| Reconciliation: dashboard numbers equal ledger | 12 | `backend/internal/ledger/reconcile_test.go` |
| axe: no critical issues on `/design` and key pages | 2, 6, 18 | Playwright plus axe |
| Reduced motion removes non-essential animation | 2, 5 | Playwright with `reducedMotion: 'reduce'` |
| Design quality gate (DESIGN.md section 12) | every UI stage | Stage report answers |
