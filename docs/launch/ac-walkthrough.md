# Acceptance criteria walkthrough (Stage 20a, 2026-11-28)

Each acceptance criterion from the brief, what proves it, and where that proof last ran. The full list of tests per criterion is in `docs/traceability.md`; this page is the evidence summary for the launch decision.

## Where the evidence comes from

| Source | What ran | Result |
|---|---|---|
| CI on `main` at `73e0a7a` (2026-11-27), [run 46062865229](https://github.com/MboyaDan/mezai/actions/runs/46062865229) | Backend (Go) against PostgreSQL, Frontend (lint, types, unit, build, e2e), Full stack (API + site + browser specs), API image build, secrets scan | All five jobs passed |
| Pilot demo on staging (brief section 18), 2026-11-19, `scripts/staging.sh demo` | Seven steps in a browser against https://mezai.vercel.app and the Render API | All seven passed; screenshots in `docs/screenshots/stage-19e/` |

Stages 27 to 30 (PRs #81 to #84) were open when this was written; each PR runs the same CI before merge.

## Criteria

| AC | What it requires | Proof | Demo step on staging |
|---|---|---|---|
| AC-01 | Every endpoint checks a permission key | `TestAuthorisationMatrix` (every role × every guarded route) and `TestEveryV1RouteDeclaresAccess`; a route missing from the matrix fails the tests | All steps: each role reached only its own screens |
| AC-02 | Lockout, reset, CSRF, audit; last Super Admin protected | `auth_test.go` (lockout, CSRF, audit), `TestPasswordReset`, `TestSuperAdminProtections` | n/a |
| AC-03 | No stat without a source; no logos, testimonials or certifications; demonstrations labelled | `content/honesty.test.ts` (source scan), `tests/e2e/honesty.spec.ts` and `pages.spec.ts` (every public route rendered) | 1: home and quality pages |
| AC-04 | Sign-up with consent version and time, profile, qualification, approval | `TestContributorOnboarding`; CI job "Full stack (AC-04)" runs `join.spec.ts` end to end with email | 3: join, test submitted, Ops approves, contributor active |
| AC-05 | Project creation; CSV/JSONL import with row errors; an invalid file imports nothing | `TestProjectImportAC05`; `projects.spec.ts` | 2: organisation, new project, items, gold, project live |
| AC-06 | Never the same item twice; exactly k contributors; gold rate within tolerance; gold indistinguishable | `TestGoldRateAndOverlap` (1,000 assignments: 101 gold at rate 0.10), `TestEachItemGetsKContributors` | 3–4: 177 submissions from sample contributors |
| AC-07 | Same idempotency key twice gives one submission and one earning | `TestTaskFlow`, `TestReviewAndAdjudication`, `TestGoldFeedbackBatch` | n/a |
| AC-08 | Gold accuracy, Cohen's and Fleiss' kappa, consensus and trust match worked examples; automatic pause below the floor | `quality/agreement_test.go` (Cohen 0.4, Fleiss 0.210), `alpha_test.go` (Krippendorff 2011: 0.743, 0.815, 0.849), `tasks/quality_test.go` | 4: contributor004 paused automatically, Ops emailed |
| AC-09 | Routing to review; adjudicated items final and immutable; correct manifest | `TestReviewAndAdjudication`, `TestDeliverableExport`, `review.spec.ts`, `deliverables.spec.ts` | 5: review queue and decision with feedback; 6: delivery downloaded, manifest and JSONL match |
| AC-10 | A client cannot read another organisation's data through the UI, API or database role | `db/rls_test.go` (`TestOrganisationIsolation` and two more), `TestClientPortal`, 27 tenancy probes (Stage 17c), `client-portal.spec.ts` | 6: client home, project and deliverables |
| AC-11 | The creator cannot approve a batch; export totals equal approved earnings; balances equal the ledger | `TestPayoutsManualPath`, `payouts.spec.ts` | 7: batch of 3 lines, KES 2,690.00, approved by a second person, settled, ledger matched |

## Not covered by this walkthrough

- **Real money:** M-Pesa B2C against Safaricom (DG-02 not cleared). The manual bulk-file path (AC-11) is the pilot's payout route; B2C has been tested only against the in-process fake.
- **Real contributors:** none have joined. DG-05 (advocate review of the privacy notice and contributor terms) must clear first.
- **Production:** no production environment exists yet. `docs/launch/checklist.md` lists the steps.
