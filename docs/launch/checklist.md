# Pilot launch checklist (Stage 20a, 2026-11-28)

What must happen before the first real client data and the first real contributors. "Done" items have evidence in the repository; the rest name who acts.

## Decision gates

| Gate | What | Status | Who |
|---|---|---|---|
| DG-05 | Advocate approves the privacy notice (0.5 once #83 merges) and contributor terms (0.4); then set `approved: true` in `frontend/content/legal-index.ts` | **Open**: blocks real contributors | Dan and the advocate |
| DG-02 | B2C-enabled shortcode and Daraja production credentials | Open: pilot pays through the M-Pesa bulk file instead | Dan, Safaricom |
| DG-01 | Public domain | Open: the site runs on `mezai.vercel.app` | Dan |
| DG-06 | SMS | Open: email only | Dan |
| DG-07 | Cloudflare Turnstile on public forms | Open: honeypot and rate limits are on | Dan |

## Environment

Production is a **new, empty database**, never a copy of staging: migrations and the first Super Admin only (CLAUDE.md section 2). Staging keeps its sample data for demos, or is emptied with `scripts/staging.sh reset` if it is to become the pilot environment.

| Step | Command or place | Status |
|---|---|---|
| 1. Create the production database (Supabase project, Frankfurt), API service (Render) and site (Vercel), following `docs/deployment.md` with production values | dashboards | Not started |
| 2. Apply migrations | in `backend/`: `go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir migrations postgres "$PROD_DATABASE_URL" up` from your machine (never automatic) | Not started |
| 3. First Super Admin | `SETUP_ADMIN_EMAIL=… SETUP_ADMIN_NAME=… SETUP_ADMIN_PASSWORD=… DATABASE_URL=… go run ./cmd/setup` in `backend/`; remove the password from the shell afterwards | Not started |
| 4. Check Supabase API roles have no access | the queries in `scripts/staging.sh check`, against production | Not started |
| 5. Production variables differing from staging | `APP_ENV=production`, `MAIL_SKIP_TEST_DOMAINS=false`, new `IDENTITY_HMAC_KEY` and `PROXY_SHARED_SECRET` (never reuse staging's), `SITE_INDEXABLE=1` on Vercel only when the domain is final, `MPESA_ENV=off` until DG-02 | Not started |
| 6. Two-step sign-in on for every staff account | `/w/account` | Not started |
| 7. Backups: daily, plus a tested restore | `scripts/staging.sh backup` and `restore-test` pattern, against production | Not started |
| 8. Remove or keep staging sample data | keep for demos, or `scripts/staging.sh reset` (backup first, asks for the host name; tested against a throwaway PostgreSQL, D-172). Files in the storage bucket are not removed: empty the staging bucket (`S3_BUCKET` in `.env.staging`) in Supabase Storage by hand | Ready |

## Content and materials

| Item | Status |
|---|---|
| Pilot demo script (brief section 18) | Done: `scripts/staging.sh demo`, all seven steps passed on staging 2026-11-19 |
| "How MezAI assures quality" one-page PDF | Done: `frontend/public/downloads/how-mezai-assures-quality.pdf`, made from `content/quality.ts` with `npm run pdf:quality`; linked from `/quality`. Regenerate whenever that file or the domain changes |
| Acceptance criteria walkthrough | Done: `docs/launch/ac-walkthrough.md` |
| Booking link, trust commitments (NDA, DPA, ODPC registration, fair pay), package prices | Waiting for Dan: the slots stay hidden until filled (D-161) |
| Google sign-in | Built (#83); switch on with the Google client (`docs/deployment.md`, "Google sign-in") |
| Data protection contact address | Waiting for Dan (Q2) |
