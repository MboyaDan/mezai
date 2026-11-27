# Security review

Started in Stage 17a (2026-11-16); retention and data subject tooling added in 17b, the tenancy review, upload checks, log review, secrets scan and threat model in 17c (2026-11-17). Optional two-step sign-in (TOTP) added in 17d. This file records what was checked, what was found and what was done. "Checked" means read in code or run; nothing here is a certification.

## One person, one account

| Question | Answer |
|---|---|
| What stops a second account? | One account per M-Pesa number and per National ID number (unique in the database), the phone interview on that number, Ops verification with possible duplicates shown, and the M-Pesa registered-name check before the first payout. |
| Is the ID number stored? | No. A keyed fingerprint of the whole number (D-136) and the last 3 digits. |
| Are the last 3 digits used to tell people apart? | No. With 1,000 possible endings, two of any 38 people are more likely than not to share one; among 50 contributors it is about 71% likely. They are shown so the person and the interviewer can confirm which number is on file. `TestLast3NeverIdentifies` and `TestIdentityHardening` prove two numbers with the same ending are two people. |
| What if the database leaks? | Fingerprints cannot be tried without `IDENTITY_HMAC_KEY`, which lives only in the environment. |
| What if the database and the key leak? | Every possible number can be tried: about 32 ms each, over a CPU-month for all of them (scheme 2). The last 3 digits shorten that for one chosen person to about an hour. Keep the key out of backups and logs; this is the residual risk of holding any matchable form of the number. |
| Can someone find out whether a person is registered? | Only by being signed in, and at most 3 answers a day per account (D-137), each recorded for Ops. |
| What if the M-Pesa line is registered to someone else? | Every direct M-Pesa payment brings back the registered name; it is compared with the profile name automatically and only the result is kept (D-139). A mismatch holds further payouts until Ops has called and recorded the name check again. The payment already made stands. |
| What is not covered? | A person using someone else's ID number and someone else's M-Pesa line, registered in the same name as the profile. The interview and the name check are the controls; a check against the national register (A-082, option b) would close it and needs DG-05. |

## Findings, Stage 17a

| # | Finding | Severity | Status |
|---|---|---|---|
| 1 | No security headers on the API or the site (framing, sniffing, content policy, HSTS) | Medium | Fixed (D-138), tested in `router_test.go`, `lib/security-headers.test.ts`, `tests/e2e/security.spec.ts` |
| 2 | Go 1.26.0 toolchain and quic-go 0.59.0: 34 known vulnerabilities reaching our code (govulncheck) | Medium | Fixed: Go 1.26.11, quic-go 0.59.1; 0 reaching our code. Weekly scan added |
| 3 | A signed-in account could test 30 ID or phone numbers an hour for registration | Medium | Fixed (D-137) |
| 4 | Fingerprint fast to compute if database and key both leak | Low (needs both) | Hardened (D-136) |
| 5 | Request size limited per handler, not globally | Low | Fixed: 1 MiB cap before every handler; uploads keep their own limits |
| 6 | Rate limits are held in memory per API instance | Low for one instance | Open: Stage 17/19 (shared store if more than one instance runs) |
| 7 | Site policy allows inline scripts | Low | Open: A-081 |
| 9 | Stored M-Pesa callbacks held the payee's registered name and full phone number, and the echo of our callback URL with its secret token | Medium | Fixed (D-139): removed before storing; migration 00024 cleans earlier rows |
| 10 | Contributor detail page scrolled sideways at 390 px for anyone with qualification attempts | Low (layout) | Fixed: the column's grid track can shrink; the table scrolls in its own frame |
| 8 | 17 moderate `npm audit` findings in development dependencies (not shipped) | Low | Open: waiting for upstream; production dependencies have 0 (re-checked 17c) |

## Checked and found sound

Sessions (hashed tokens, HttpOnly, Secure, SameSite=Lax, idle and absolute expiry), CSRF (origin checks and token), Argon2id passwords with a 12-character minimum, permission check on every route (`TestAuthorisationMatrix`), row-level security for client data, phone masking, uploads capped and read through one function, M-Pesa callbacks (token, optional address list, matched before use), no secrets or full phone numbers in logs (checked during the Stage 15a run).

## Retention and data subject requests (Stage 17b)

| Question | Answer |
|---|---|
| When does client task content go? | `retention_days` after the project closes (default 90), checked hourly; or at once when Ops with `privacy.manage` types the project name and records who asked (D-142). |
| What goes, what stays? | Item text, every answer and final answer, gold answers, review and feedback text, rejection reasons, notes, import errors, and the stored files (deliverables, imports, request sample). Counts, quality figures, earnings and the audit trail stay. Qualification tests copied from the project's gold are retired and blanked (A-089). |
| Can the app rewrite answers to "purge" them some other way? | No. The append-only grants are unchanged; only `purge_project_content` (SECURITY DEFINER) can blank them, and `TestRetentionPurge` proves the app role is still refused. |
| What if file deletion fails? | The key stays in `storage_deletions` with the error and the hourly sweep retries it (up to 20 times, then it waits for a person). |
| How does a contributor see their data? | `GET /v1/me/data-export`: one JSON file, 5 an hour, audited; Ops can produce the same for an outside request (D-143). |
| What does erasure keep? | The public code, ledger entries, payout amounts and receipts, submissions (the client's dataset), status history and audit actions. Name, email, numbers, profile, CV, interviews, sessions and IPs go; the payee on payout lines becomes `erased`. |
| Can erasure lose someone's pay? | No: it is refused while any money is owed either way, a payout line is pending, or answers await a decision (A-088). |
| Other data with no further use | Declined request samples after the same period as closed projects, sent emails' merge data after 30 days, expired idempotency records (D-144). |
| Not covered | Database backups still hold erased or purged data until they expire: the backup retention period is set in Stage 19 and must be stated in the privacy notice. Mailpit (local only) and the email provider's own logs hold sent emails. |

## Findings, Stage 17c

| # | Finding | Severity | Status |
|---|---|---|---|
| 11 | A contributor asking for the next task on an unpublished (draft or ready) project of any organisation got 409 "not live", confirming the project exists | Low (IDs are random UUIDs) | Fixed: 404 like every other unseen project. `TestCrossTenantMatrix` |
| 12 | A PDF upload (CV, invoice) was accepted on its first five bytes; a PDF with scripts, launch actions or embedded files reached Ops, who open them in desktop readers | Medium | Fixed (D-145): whole file and no active content names. Compressed object streams are not unpacked, so this is a filter, not a scanner |
| 13 | A project request "sample" was checked by extension only: a web page, SVG or binary renamed `.csv` was stored and offered for download | Low (downloads are attachments with `nosniff`) | Fixed (D-145): UTF-8 text, no NUL bytes, not markup |
| 14 | Error text from outside services is logged as is: an SMTP rejection quotes the recipient's address; M-Pesa and database errors can carry numbers | Medium | Fixed (D-146): every log line and stored email error passes through a redacting handler |
| 15 | No secrets scan | Medium | Fixed (D-147): gitleaks over the whole history on every PR; one test fixture allowlisted by exact value; history clean |
| 16 | Local `node_modules` held Next.js 16.4.4 with a known advisory; the lockfile already pins 16.4.6 | Low (local only) | Fixed by `npm ci`; CI installs from the lockfile |

## Tenancy review (Stage 17c)

Every route was read for how it finds its rows. Staff routes see every organisation by design and are guarded by permission keys (`TestAuthorisationMatrix`). For the rest, the review is a test that keeps running:

- `TestCrossTenantMatrix` takes from the router every route a client, contributor or reviewer can call with an `:id` (27 routes) and calls each as the wrong person with a real ID that belongs to someone else: another organisation's client admin, another contributor, an unassigned reviewer. Each must answer 403 or 404 with no data, and the rightful caller must not be refused (so the ID was real). A new route without a probe fails the test.
- `TestCrossTenantParameters` passes another organisation's ID in query and body parameters (`org_id` on projects, activity, team, invoices, invitations, project requests; `project_id` on the review queue). Nothing leaks and nothing is created in the other organisation.
- Marking someone else's feedback read answers 204 (the route is idempotent), but the update is scoped to the caller and changes nothing; the test checks the row.
- Below the API, row-level security enforces the same split again (`TestOrganisationIsolation` and the other `rls_test.go` tests). `TestEveryOrgTableHasRLS` requires a policy on every table with an `org_id`, with four named exceptions.

## Logs (Stage 17c)

Read every log call. Request lines carry the route template, method, status, duration and request ID, never the path, query or body. Other lines carry IDs, counts, kinds and errors. Errors are the risk (finding 14), so the logger now masks email addresses (domain kept) and Kenyan mobile numbers (last three digits kept) everywhere (`pii.RedactHandler`, `TestRedactHandler`). Passwords, session tokens, CSRF secrets and task content are not passed to any log call. Seed and setup commands print sample or the operator's own admin email to the terminal, not to the API log.

## Threat model (pilot)

**What we protect:** client task content and deliverables; contributor personal data (contact details, M-Pesa number, ID fingerprint, CV, interview notes); money (ledger, payout batches, M-Pesa credentials); the integrity of quality figures (gold answers, consensus, trust); staff accounts.

**Who might attack:** an outsider on the internet; a contributor (gaming quality or pay, scraping content, a second account); a client user (looking at another client); a curious or compromised staff account; someone who obtains a database dump, backup or log export.

**Trust boundaries:** browser to the site (Next.js, rewrite to `/api`); site to Go API; API to PostgreSQL (non-owner `mezai_app` role, row-level security); API to object storage (signed, server-side only); API to Daraja and SMTP; the M-Pesa callback endpoint, the one place the internet calls us unauthenticated.

| Threat | Controls in place | Residual risk and next step |
|---|---|---|
| Account takeover by password guessing or stuffing | Argon2id, 12-character minimum, per-IP and per-email limits, lockout, audited sign-ins; optional two-step sign-in for staff (D-148) with encrypted secrets, one-time codes and recovery codes, wrong codes counting towards lockout | Two-step is optional (A-091): recommend making it required for Super Admins and Payout Approvers before production. Rate limits are in memory per instance (finding 6). |
| Session theft or cross-site requests | HttpOnly, Secure, SameSite=Lax cookie; hashed tokens; idle and absolute expiry; CSRF token and origin check; strict headers and content policy | Inline scripts allowed on the site (A-081). |
| A client reads another client's data | Org scope in every query, RLS on client tables, the two cross-tenant tests | A new client-owned table must get a policy: `TestEveryOrgTableHasRLS` fails for any table with `org_id` and no row-level security unless it is a named exception (audit log, enquiries, invitations, user roles, each with its reason). |
| A contributor reads content beyond their assigned item, or the gold answers | Item-by-item serving, contributor scope, gold indistinguishable, delayed batched gold feedback, export excludes qualification answers | A contributor can copy what they are shown; contributor terms carry confidentiality (brief 12). |
| A contributor games quality or pay (second account, scripted answers, double submit) | One account per M-Pesa number and ID fingerprint, interview, name check on payment, gold mixing and trust auto-pause, idempotent submit, QA checks | Someone using another person's ID and line in that person's name (see "One person, one account"). |
| Money paid twice or to the wrong person | Two-person approval, idempotency keys, B2C mark-then-send, status queries before retry, callbacks matched and stored before use, name comparison, append-only ledger, reconciliation | Daraja production not yet exercised (DG-02). M-Pesa credentials live in environment variables; anyone with server access has them. |
| Forged M-Pesa callbacks | Secret token in the callback URL, optional source address list, match on conversation IDs and amount, stored with a verdict | The token is in the URL, so it appears in Daraja's configuration; rotate it if exposed. |
| Malicious uploads | Size caps, type checks inside the file (D-145), stored outside the web root, served only as attachments with `nosniff` and a deny-all policy | No malware scanning; Ops should open CVs and invoices in a browser's PDF viewer rather than a desktop reader. |
| Data exposure from a dump, backup or log | ID numbers never stored (keyed fingerprint), phone numbers masked in lists and logs, log redaction, retention and erasure (17b), secrets out of git (scan) | Backups keep erased and purged data until they expire: set and state the backup period in Stage 19. A dump plus `IDENTITY_HMAC_KEY` allows matching ID numbers. |
| A compromised or careless staff account | Permission keys per action, Super Admin protections, audit log on every write, two-person payouts, purge and erasure need typed confirmation and a reason | Ops Managers can read all contributor data by design; two-step sign-in is optional (A-091); no alerting on unusual staff activity yet. |
| Vulnerable dependencies | Dependabot, weekly govulncheck, `npm audit` (0 production findings) | Development-only npm advisories (finding 8). |
| Denial of service | Request size cap, per-route limits, statement and lock timeouts | No edge protection until Cloudflare is in front (Stage 19). |

## Two-step sign-in (Stage 17d)

| Question | Answer |
|---|---|
| Who can use it? | Staff accounts (Super Admin, Ops Manager, Payout Approver, Reviewer). Optional (A-091). |
| What if the database leaks? | Secrets are encrypted with a key derived from `IDENTITY_HMAC_KEY`, kept only in the environment; recovery codes are hashes. |
| Can a code be replayed? | No: the last used time step is stored and the user row is locked while a code is checked; recovery codes are marked used in the same transaction. |
| Can codes be guessed? | Five wrong codes close the sign-in step, each counts towards the 5-failure lockout, and the step shares the per-IP sign-in limit. |
| Lost phone? | A recovery code, or a reset by a `user.manage` holder with a reason in the audit log (a Super Admin's only by a Super Admin). |
| Tested | `TestTOTPRFCVectors` (RFC 6238 appendix B), `TestVerifyTOTP`, `TestSealer`, `TestRecoveryCodes`, `TestTwoStepSignIn`, `TestTwoStepLimitsAndReset`, `tests/full/two-step.spec.ts`. |

## Google sign-in (Stage 29, D-171)

| Question | Answer |
|---|---|
| Who can use it? | Contributors without two-step sign-in. Staff and client accounts are refused and keep the password (and code) sign-in. |
| How is the answer from Google trusted? | Authorization code flow with PKCE (S256) and a nonce, run by the API, not the browser. The ID token is checked by `go-oidc` against Google's published keys: signature, issuer, audience (our client ID), expiry and nonce. `email_verified` must be true. |
| Can someone else's sign-in be pushed into my browser (login CSRF)? | No: the callback needs the state in the address and the same value in an HttpOnly cookie set when this browser left; the database keeps only the state's hash, for 10 minutes, and uses it once. |
| Pre-registered account in my name? | Linking by email clears the password of an account whose email was never verified, so whoever set it cannot use it after the real owner arrives through Google. |
| What is kept? | Google's account ID and the email it reported, in `user_identities`. No access or refresh token, no photo. Export includes it; erasure deletes it. |
| Tested | `TestGoogleSignIn` with a test OpenID provider that signs real RS256 tokens (sign-up, sign-in, linking, staff refused, unverified email, wrong nonce, missing or reused state, cancel), `TestGoogleOff`, `TestContributorExportAndErasure`, `tests/full/google-sign-in.spec.ts`. Locally the start route was checked against Google's real discovery document. |

## Still to do

Stage 19: shared rate-limit store if more than one API instance runs, backup period, edge protection.
