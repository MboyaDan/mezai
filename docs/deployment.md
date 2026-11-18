# Deploying MezAI to staging

Stage 19a. Follow the steps in order; each ends with a check. Dashboards change their wording from time to time: if a button is named differently, look for the nearest match and note it in the PR.

**What runs where**

| Piece | Host | Plan | Address |
|---|---|---|---|
| Database (PostgreSQL 17) | Supabase, region Central EU (Frankfurt) | Free | session pooler, port 5432 |
| File storage (S3 protocol) | Supabase Storage, same project | Free (1 GB) | `https://<ref>.storage.supabase.co/storage/v1/s3` |
| Go API (Docker) | Render, region Frankfurt | Free | `https://mezai-api-staging.onrender.com` |
| Website and portals (Next.js) | Vercel | Hobby | `https://mezai-staging.vercel.app` |
| Outgoing email (SMTP) | Brevo | Free (300 a day) | `smtp-relay.brevo.com:2525` |
| Uptime ping | UptimeRobot | Free | pings `/healthz` every 5 minutes |

The browser only talks to Vercel. Vercel forwards `/api/*` to Render. Render is the only thing that talks to the database and storage. Why these choices: D-152 to D-154 in `docs/DECISIONS.md`.

**Free tiers are for staging only** (CLAUDE.md section 2). What must change before real client data is at the end of this file.

**Keep a password manager open.** Every secret below goes there and into the dashboard that needs it, never into the repository, chat or email.

---

## Step 1. Supabase project (database)

1. Go to **supabase.com**, click **Start your project**, sign in with GitHub.
2. If asked for an organisation, create one: name `ELS Software Solutions`, type **Company**, plan **Free**.
3. Click **New project** and fill in:
   - **Project name:** `mezai-staging`
   - **Database password:** click **Generate a password**, then copy it to your password manager. Use letters and digits only (regenerate if it has symbols): the password goes inside a URL.
   - **Region:** **Central EU (Frankfurt)** (`eu-central-1`).
   - If the form shows a **Security** or **Data API** option, choose **Only connection string** (no Data API). MezAI never uses Supabase's automatic API.
4. Click **Create new project** and wait about two minutes until it shows **Healthy**.
5. **Turn the Data API off** (if step 3 did not): left menu **Project Settings → Data API**, switch **Enable Data API** off and save. Migration `00028` also takes away every grant that API could use, so this is a second lock.
6. **Copy the connection string:** click **Connect** (top bar) → **Connection String** → **Type: URI** → **Method: Session pooler**. It looks like:

   ```
   postgresql://postgres.abcdefghijklmnop:[YOUR-PASSWORD]@aws-0-eu-central-1.pooler.supabase.com:5432/postgres
   ```

   Replace `[YOUR-PASSWORD]` with the password from step 3 and add `?sslmode=require&pool_max_conns=5` to the end. Save the result in your password manager as **MezAI staging DATABASE_URL**.

   Use the **session pooler**, not the direct connection (IPv6 only, Render cannot reach it) and not the transaction pooler on port 6543 (it breaks the API's prepared statements and `SET LOCAL ROLE`).

**Check:** left menu **SQL Editor** → run `select version();`. It should say PostgreSQL 17 (17.12 on 2026-11-18). If it says 18, tell me: the backup script's `PG_IMAGE` must change to `postgres:18-alpine`.

## Step 2. Supabase Storage (files)

Same project.

1. Left menu **Storage** → **New bucket**. Name `mezai-staging`. **Public bucket: off.** Leave the size and type limits empty (the API checks every upload itself). Create.
2. In **Storage**, open **Settings** (or **S3 Configuration**). Make sure **S3 protocol connection** is on. Copy:
   - **Endpoint** (like `https://abcdefghijklmnop.storage.supabase.co/storage/v1/s3`)
   - **Region** (like `eu-central-1`)
3. In the same page under **S3 Access Keys**, click **New access key**, description `render-api`, create. Copy the **Access key ID** and **Secret access key** now: the secret is shown once.

**Check:** you have four values: endpoint, region, access key ID, secret access key. Bucket name is `mezai-staging`.

## Step 3. Brevo (email)

Staging has no domain yet (DG-01), so email goes out from an address you own through Brevo. It may land in spam; that is fine for staging.

1. Go to **brevo.com**, sign up (free plan). Finish the account questions; Brevo may take a few hours to activate sending for a new account.
2. Top-right menu → **Senders, Domains & Dedicated IPs** → **Senders** → **Add a sender**. From name `MezAI Staging`, email: an address you read. Confirm the code Brevo emails you.
3. Top-right menu → **SMTP & API** → tab **SMTP** → **Generate a new SMTP key**, name `render-staging`. Copy:
   - **Login** (like `8a1b2c001@smtp-brevo.com`)
   - **SMTP key** (the password; shown once)

Brevo's free plan (300 emails a day, no card) is enough for staging: pick **Free** when Brevo offers plans. Port **2525** is on purpose: Render's free plan blocks outgoing mail on ports 25, 465 and 587. Whether 2525 is open on Render is not confirmed; if sending times out (`mail` errors in Render **Logs**), the fallback is Brevo's HTTPS API on port 443, which needs a small API change (19b).

## Step 4. Prepare the database from your laptop

The API never migrates the database by itself (CLAUDE.md section 2). You run migrations from your machine with the Go toolchain and Docker you already use.

1. Make sure your local copy is on the merged `main`:

   ```sh
   cd ~/work_k/mezai && git switch main && git pull --ff-only
   ```

2. Create the settings file (git-ignored) and fill it in:

   ```sh
   cp backend/.env.staging.example backend/.env.staging
   open -e backend/.env.staging
   ```

   - `DATABASE_URL`: from step 1.6.
   - `SETUP_ADMIN_EMAIL`, `SETUP_ADMIN_NAME`, `SETUP_ADMIN_PASSWORD`: your own Super Admin (an address you read; a new, long password).
   - `SEED_PASSWORD`: a new password of 16+ characters. The repository is public, so the built-in sample password is public too.

   One `NAME=value` per line; values may hold spaces and symbols (the script reads the file, it does not run it).

3. Run, from the repository root:

   ```sh
   backend/scripts/staging.sh migrate   # applies 00001 to 00028
   backend/scripts/staging.sh setup     # creates your Super Admin
   backend/scripts/staging.sh seed      # sample data for the demo (all marked "(Sample)")
   ```

4. Delete the `SETUP_ADMIN_PASSWORD` line from `backend/.env.staging`.

**Check:** `backend/scripts/staging.sh status` lists every migration as applied, up to `00028_hosted_api_roles.sql`. `backend/scripts/staging.sh check` shows 0 grants and 0 default privileges for Supabase's API roles from our role, and over 200 for `mezai_app` (the rules it lists for `supabase_admin` and Supabase's own schemas do not reach MezAI's tables). The seed prints `19 users, 2 organisations, 12 contributors, 4 projects ...`.

Sample accounts (`superadmin@sample.invalid`, `ops@sample.invalid`, `reviewer@sample.invalid`, `contributor001@sample.invalid` and so on) sign in with your `SEED_PASSWORD`. Their emails go nowhere (`.invalid`). Stage 20 removes them.

## Step 5. Render (the API)

Render reads `render.yaml` from `main`, so the Stage 19a PR must be merged first.

1. Make two secrets now and save them in your password manager:

   ```sh
   openssl rand -base64 32   # MezAI staging IDENTITY_HMAC_KEY
   openssl rand -hex 32      # MezAI staging PROXY_SHARED_SECRET
   ```

   Never change the identity key once real contributors join (stored National ID fingerprints would stop matching). The proxy secret goes into Render and Vercel with the same value (D-156).

2. Go to **dashboard.render.com**, sign in with GitHub.
3. **New** (top right) → **Blueprint**. Connect GitHub if asked, and give Render access to the `MboyaDan/mezai` repository only.
4. Pick `MboyaDan/mezai`. **Blueprint name:** `mezai-staging`. **Branch:** `main`. Render shows one service, `mezai-api-staging`, and asks for the secret values:

   | Variable | Value |
   |---|---|
   | `APP_ORIGIN` | `https://mezai-staging.vercel.app` (fixed in step 7 if Vercel gives another address) |
   | `DATABASE_URL` | from step 1.6, with `?sslmode=require&pool_max_conns=5` |
   | `S3_ENDPOINT` | from step 2.2 |
   | `S3_REGION` | from step 2.2 (e.g. `eu-central-1`) |
   | `S3_BUCKET` | `mezai-staging` |
   | `S3_ACCESS_KEY_ID` | from step 2.3 |
   | `S3_SECRET_ACCESS_KEY` | from step 2.3 |
   | `IDENTITY_HMAC_KEY` | from step 5.1 |
   | `PROXY_SHARED_SECRET` | from step 5.1 (the same value goes into Vercel) |
   | `SMTP_ADDR` | `smtp-relay.brevo.com:2525` |
   | `SMTP_USERNAME` | Brevo login from step 3.3 |
   | `SMTP_PASSWORD` | Brevo SMTP key from step 3.3 |
   | `MAIL_FROM` | `MezAI Staging <the sender address from step 3.2>` |
   | `ENQUIRY_NOTIFY_TO` | the inbox that should get project enquiries |

   The other variables (`APP_ENV=staging`, `PORT`, `TRUSTED_PROXIES`, `MPESA_ENV=off` and so on) come from `render.yaml`; do not add them.

5. Click **Apply**. The first Docker build takes 3 to 6 minutes. Watch **Logs**: the last lines should be JSON with `"msg":"api listening"` and `"env":"staging"`, and no `config` errors.

**Check:** open `https://mezai-api-staging.onrender.com/healthz` (should answer ok) and `/readyz` (ok means it reached the database **and** the bucket). If Render gave the service another address (it adds a suffix when the name is taken), use that address everywhere below.

If `/readyz` fails, its answer and the log name the check that failed: `database` means the `DATABASE_URL` (password, pooler host, the `?sslmode=require` part); `storage` means the S3 values.

## Step 6. Vercel (the website)

1. Go to **vercel.com**, sign up with GitHub (Hobby plan).
2. **Add New…** → **Project** → **Import** next to `MboyaDan/mezai` (use **Adjust GitHub App permissions** to allow this repository if it is not listed).
3. On the **Configure Project** screen:
   - **Project Name:** `mezai-staging`
   - **Framework Preset:** Next.js (detected)
   - **Root Directory:** click **Edit**, choose `frontend`, **Continue**.
   - **Build and Output Settings:** leave as they are.
   - **Environment Variables:** add

     | Name | Value |
     |---|---|
     | `API_ORIGIN` | `https://mezai-api-staging.onrender.com` (no slash at the end) |
     | `SITE_URL` | `https://mezai-staging.vercel.app` |
     | `PROXY_SHARED_SECRET` | the same value as on Render |

     Do **not** add `SITE_INDEXABLE` (staging must stay out of search engines) or `NEXT_PUBLIC_TURNSTILE_SITE_KEY` (DG-07).
4. **Deploy**. About 3 minutes.
5. **Settings → Build and Deployment → Node.js Version:** `24.x` (the frontend needs Node 24). If you changed it, **Deployments → ⋯ → Redeploy**.

`API_ORIGIN` is read when the site is built, so after any change to it, redeploy.

**Check:** the address Vercel shows under **Domains**. If it is not exactly `https://mezai-staging.vercel.app`, go to step 7.

## Step 7. Connect the two and check the proxy

1. If the Vercel address differs: Render → `mezai-api-staging` → **Environment** → set `APP_ORIGIN` to the real Vercel address → **Save, rebuild and deploy**. Vercel → **Settings → Environment Variables** → set `SITE_URL` to the same → **Deployments → Redeploy**.
2. Preview deployments (one per pull request) have other addresses, so sign-in and forms are refused there by design. Test on the production address only.

## Step 8. Uptime ping

A free Render service sleeps after 15 minutes without traffic; then the job runner (emails, nightly quality jobs) stops and the first visitor waits up to a minute. A ping keeps it awake (one service fits in Render's 750 free hours a month) and also stops Supabase pausing the project after a week without use.

1. Go to **uptimerobot.com**, sign up (free).
2. **New monitor** → type **HTTP(s)** → name `MezAI staging API` → URL `https://mezai-api-staging.onrender.com/healthz` → interval **5 minutes** → alert contact: your email → **Create monitor**.
3. Optional second monitor for `https://mezai-staging.vercel.app/`.

## Step 9. Smoke test (then tell me the results)

On `https://mezai-staging.vercel.app`, in a private window:

1. Home page loads with every section; open one inner page (Services) too.
2. **Sign in** with your Super Admin from step 4. (Proves cookies, CSRF and the origin check through Vercel's proxy.) Turn on two-step sign-in at `/w/account` if you like.
3. Send the **Contact** form. The enquiry email should reach `ENQUIRY_NOTIFY_TO` within a minute or two (check spam). (Proves SMTP.)
4. Sign in as `ops@sample.invalid`, open a sample project, upload a small CSV of items. (Proves storage.)
5. Sign in again, then Supabase → **SQL Editor** → run `select created_at, action, ip from audit_log where ip is not null order by created_at desc limit 5;` and note the IP of your sign-in. Compare with your public IP (search "what is my IP"). They must match. If they show `::1`, `PROXY_SHARED_SECRET` is missing or differs between Render and Vercel (D-156).
6. From your laptop: `backend/scripts/staging.sh backup` then `backend/scripts/staging.sh restore-test`. It should end with `row counts match`.

Send me: which of 1 to 6 passed, the two IPs from 5, and any error text from Render **Logs**. Do not paste secrets.

---

## Environment variables

### API (Render)

| Variable | Staging value | Secret | Notes |
|---|---|---|---|
| `APP_ENV` | `staging` | no | `render.yaml` |
| `PORT`, `HTTP_ADDR` | `8080`, `:8080` | no | `render.yaml` |
| `LOG_LEVEL`, `LOG_FORMAT` | `info`, `json` | no | `render.yaml` |
| `APP_ORIGIN` | Vercel address | no | must match exactly (no slash) |
| `COOKIE_SECURE` | `true` | no | refused if false outside development |
| `TRUSTED_PROXIES` | `127.0.0.1` | no | `render.yaml`; visitor addresses come from the site (D-156) |
| `PROXY_SHARED_SECRET` | `openssl rand -hex 32` | **yes** | same value on Vercel; required outside development |
| `SESSION_TTL`, `SESSION_IDLE` | `168h`, `48h` | no | |
| `DATABASE_URL` | session pooler URL | **yes** | `sslmode=require&pool_max_conns=5` |
| `S3_ENDPOINT`, `S3_REGION`, `S3_BUCKET` | Supabase Storage | no | |
| `S3_USE_PATH_STYLE` | `true` | no | |
| `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | Supabase S3 key | **yes** | |
| `IDENTITY_HMAC_KEY` | `openssl rand -base64 32` | **yes** | never rotate after contributors join |
| `SMTP_ADDR` | `smtp-relay.brevo.com:2525` | no | |
| `SMTP_USERNAME`, `SMTP_PASSWORD` | Brevo SMTP login and key | **yes** | |
| `MAIL_FROM` | `MezAI Staging <verified sender>` | no | |
| `ENQUIRY_NOTIFY_TO` | ops inbox | no | |
| `TURNSTILE_SECRET_KEY` | unset | yes | DG-07 |
| `MPESA_ENV` | `off` | no | DG-02 not cleared; never `fake` outside development |

### Website (Vercel)

| Variable | Staging value | Notes |
|---|---|---|
| `API_ORIGIN` | Render address | read at build time; redeploy after a change |
| `SITE_URL` | Vercel address | canonical links, sitemap, Open Graph |
| `PROXY_SHARED_SECRET` | same as Render | secret; server only, never `NEXT_PUBLIC_` |
| `SITE_INDEXABLE` | unset | unset means `noindex` and a closed `robots.txt` |
| `NEXT_PUBLIC_TURNSTILE_SITE_KEY` | unset | DG-07 |

### Laptop (`backend/.env.staging`, git-ignored)

`DATABASE_URL`, `SETUP_ADMIN_*` (password removed after setup), `SEED_PASSWORD`, optional `BACKUP_DIR` (default `~/mezai-backups`).

## Error monitoring

Staging uses Render's **Logs** (JSON, searchable, 7 days on the free plan) and Vercel's **Logs**, plus the UptimeRobot alert. Recommended before the pilot: Sentry (free Developer plan) for the API and the site. It is not wired in yet: it sends error data to a third party (a Data Protection Act question for DG-05), and the site's content security policy must allow it. Proposed for Stage 19b once you agree.

## Before real client data (what must change)

1. **Paid tiers.** Render Starter or higher (no sleeping, no SMTP block); Supabase Pro (daily backups you can restore, point-in-time recovery option, no pausing); Vercel Pro (Hobby is for non-commercial use).
2. **Domain (DG-01).** Cloudflare DNS for the site and API, HTTPS on both, `APP_ORIGIN` and `SITE_URL` changed, `SITE_INDEXABLE=1` only for production.
3. **Email.** A sending domain with SPF, DKIM and DMARC (A-086 picks the provider).
4. **Data location.** Everything here sits in Frankfurt. Personal data leaving Kenya needs the safeguards in the Data Protection Act 2019 (cross-border transfer): confirm with the advocate under DG-05 before contributors join.
5. **Separate production.** A new Supabase project, Render service and Vercel project; new secrets (never reuse staging's `IDENTITY_HMAC_KEY` or keys); no seed; migrations run by hand with the command in the PR notes.
6. Error monitoring in, two-step sign-in required for Super Admins and payout approvers (A-091).
7. **Sample data removed** (Stage 20).
