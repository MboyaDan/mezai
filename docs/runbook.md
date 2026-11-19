# Staging runbook

Day-to-day operation of the staging deployment described in `docs/deployment.md`. Commands run from the repository root with `backend/.env.staging` filled in.

## Deploying a change

1. Merge the pull request on GitHub.
2. Render and Vercel both deploy `main` automatically (3 to 6 minutes for Render, about 3 for Vercel).
3. **If the PR adds a migration**, the PR notes say so. Run it **before** the new API starts using it, normally right after merging:

   ```sh
   git switch main && git pull --ff-only
   backend/scripts/staging.sh status
   backend/scripts/staging.sh migrate
   ```

   Migrations are written so the previous API version keeps working on the new schema (add, then use, then remove in a later stage).
4. Check `https://mezai-api-staging.onrender.com/readyz` and the site's home page.

## Rolling back

- **API:** Render → `mezai-api-staging` → **Events** → pick the last good deploy → **Rollback**. Auto-deploy stays on: the next merge to `main` deploys again.
- **Website:** Vercel → **Deployments** → the last good one → **⋯ → Instant Rollback**.
- **Database:** migrations are not rolled back on a live database. Write a new migration that undoes the change (compensating, like the ledger). For a damaged database, restore from a backup (below).

## Backups

Supabase's free plan keeps no backups you can download. Take one before every migration and at least weekly:

```sh
backend/scripts/staging.sh backup         # writes ~/mezai-backups/mezai-staging-<UTC time>.dump
backend/scripts/staging.sh restore-test   # restores the newest one into a throwaway PostgreSQL and compares row counts
```

Backups contain staging data (sample data now; real people's data later). Keep `~/mezai-backups` on an encrypted disk, never in the repository, cloud drives or chat. Delete old ones (keep the last four).

**Restoring into Supabase** (after data loss): create a fresh Supabase project (or empty the public schema), then

```sh
docker run --rm -v ~/mezai-backups:/b postgres:17-alpine \
  pg_restore --no-owner -d "<session pooler URL of the target>" /b/<file>.dump
```

Restoring into plain PostgreSQL instead (not Supabase) needs the `pg_attver` lines removed, as `restore-test` does. Then point Render's `DATABASE_URL` at it, redeploy, and run the smoke test (`docs/deployment.md` step 9). Run step 1.5 (Data API off) on the new project.

## Secrets

| Secret | Where it lives | Rotate |
|---|---|---|
| Database password | Supabase → **Project Settings → Database → Reset database password** | Update `DATABASE_URL` in Render and `backend/.env.staging` |
| Storage keys | Supabase → **Storage → Settings → S3 Access Keys** | Create a new key, update Render, then delete the old key |
| Brevo SMTP key | Brevo → **SMTP & API** | Generate a new key, update Render, delete the old one |
| `IDENTITY_HMAC_KEY` | Render only | **Do not rotate** once contributors have joined (D-078) |
| `PROXY_SHARED_SECRET` | Render and Vercel (same value) | Set the new value on both, deploy Render, then redeploy Vercel; in between, visitor addresses show as `::1` (sign-in limits shared) for a few minutes |

After changing a Render variable choose **Save, rebuild and deploy**. Sessions survive an API restart (they are in the database).

## Known behaviour on free plans

- **Cold start:** if the uptime ping stops, the API sleeps after 15 minutes; the first request then takes up to a minute and may show an error page through Vercel. Reload after a minute.
- **Supabase pause:** a free project with no activity for 7 days is paused. Restore it from the Supabase dashboard (**Restore project**); data is kept.
- **Email:** 300 a day on Brevo's free plan. Messages wait in the outbox and are retried if Brevo refuses.
- **Logs:** Render keeps 7 days on the free plan.

## Sample data

Staging holds the seed's sample data for the pilot demo. Reset it by restoring a backup taken right after seeding, or (Stage 20) remove it before real users join.

## Running the pilot demo

The brief's demo script (section 18) runs in a browser against the live site:

```sh
cd backend && scripts/staging.sh demo            # all steps on https://mezai.vercel.app
scripts/staging.sh demo -g "[234]\. "             # some steps only
scripts/staging.sh demo --headed                  # watch it
```

It needs Docker (for psql) and `SEED_PASSWORD` in `backend/.env.staging`. The seven steps: visitor and enquiry; Ops set-up; a new contributor to active; contributors working (one paused automatically); review and feedback; the client's sign-in, quality view and delivery download; a payout batch approved by a second person. Each run makes a new organisation, project and contributor, all marked "(Sample)", and leaves them on staging so you can open them afterwards. Screenshots go to `docs/screenshots/stage-19e/`.

- The contact form takes **5 enquiries an hour** from one connection, and each staff account **10 sign-ins in 15 minutes**. One run uses 1 enquiry and 5 Ops sign-ins; the runner waits if a limit is reached.
- Mail to `.invalid` sample addresses is not sent (D-159), so the runner confirms its new contributor's email with a one-time token it writes to the database. In a live sales demo, sign up with a real inbox instead.
- The runner sets `trust.min_gold_results` to 5 on staging (audited) so the automatic pause shows with a 10-item gold set. Production keeps 20.
- `contributor004` is the careless one and ends **paused**. The next run reinstates them first.
- Payouts cover whole days up to **yesterday** (Nairobi), so step 7 pays what was approved before today. If nothing is due, or the last batch already covers yesterday, the step is skipped and says why. Each run marks every line paid with a made-up `DEMO…` receipt.
- Its new client user is invited through the Organisations screen and accepted with a token the runner writes, since `.invalid` mail is not sent.

## Something is wrong

1. `/healthz` fails: the API is down. Render → **Logs**, look for `config` errors or panics; roll back if a deploy caused it.
2. `/readyz` fails: database or storage unreachable. Check Supabase status (status.supabase.com) and the project's health.
3. Site loads but every sign-in or form is refused (403): `APP_ORIGIN` on Render does not match the address in the browser exactly (scheme, host, no slash).
4. Emails do not arrive: the API logs `mail` errors; check Brevo **Transactional → Logs** and the sender's verification.
