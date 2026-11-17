# Security review

Started in Stage 17a (2026-11-16); retention and data subject tooling added in 17b (2026-11-17). The threat review and TOTP are Stage 17c. This file records what was checked, what was found and what was done. "Checked" means read in code or run; nothing here is a certification.

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
| 8 | 17 moderate `npm audit` findings in development dependencies (not shipped) | Low | Open: waiting for upstream; production dependencies have 0 |

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

## Still to do (Stage 17c)

Endpoint-by-endpoint tenancy review, upload content checks, optional TOTP for Ops and payout roles, log review, secrets scan in CI, written threat model.
