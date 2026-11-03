# MezAI

AI data operations platform of ELS Software Solutions, Nairobi (pilot). One repo, two apps:

| Folder | What | Runs with |
|---|---|---|
| `backend/` | Go API (Gin, pgx, sqlc, goose), PostgreSQL, S3 storage, mail | Docker Compose |
| `frontend/` | Next.js (App Router, TypeScript, Tailwind v4): public site, contributor portal, workspace | `npm run dev` |

Docs: [`docs/PROGRESS.md`](docs/PROGRESS.md) (current state), [`docs/design/DESIGN.md`](docs/design/DESIGN.md) (design), [`CLAUDE.md`](CLAUDE.md) (how we work and the stages).

## Requirements

- Docker with Compose v2
- Go 1.26 (only for running tests and tools on the host; `go` auto-downloads the toolchain if yours is older)
- Node.js 24 and npm

## Run locally

```sh
# 1. Backend: api, postgres, storage (SeaweedFS, S3-compatible) and mailpit
make up            # creates backend/.env from .env.example on first run
curl localhost:8080/readyz

# 2. Frontend (new terminal)
cd frontend
cp .env.example .env.local   # optional; defaults to http://localhost:8080
npm install
npm run dev        # http://localhost:3000 shows "API connected"
```

| Service | URL |
|---|---|
| Website | http://localhost:3000 |
| API (direct) | http://localhost:8080/healthz, `/readyz` |
| API (through the site) | http://localhost:3000/api/healthz |
| PostgreSQL | `localhost:5433` (user `mezai`, see `backend/.env`) |
| S3 storage | http://localhost:8333 (bucket `mezai-dev`) |
| Mailpit | http://localhost:8025 |

Ports are set in `backend/.env`. PostgreSQL uses 5433 on the host so it does not clash with a local install.

## Common tasks

From the repo root (forwarded to `backend/Makefile`):

| Command | Does |
|---|---|
| `make up` / `make down` | Start or stop the backend stack (data is kept) |
| `make test` | Go tests, including PostgreSQL tests against the compose database |
| `make lint` | gofmt and go vet |
| `make migrate` | Apply goose migrations to the local database |
| `make setup` | Create the first Super Admin from `SETUP_ADMIN_*` (any environment; no-op once one exists) |
| `make seed` | Load sample data (development and test only; a second run changes nothing) |
| `make sqlc` | Generate Go code from SQL queries (CI fails if it is out of date) |
| `make logs` | Follow API logs |

Frontend, in `frontend/`:

| Command | Does |
|---|---|
| `npm run lint` · `npm run typecheck` · `npm run format:check` | Static checks |
| `npm test` | Vitest unit tests |
| `npm run build` | Production build |
| `npm run test:e2e` | Playwright smoke tests (starts the dev server if needed) |

## Rules worth knowing

- Never commit `.env` files, secrets, real personal or client data, or database dumps.
- The browser only calls `/api/*` on the site origin; the backend is the only database client.
- `backend/openapi.yaml` is the API contract; change it in the same PR as the endpoint.
- Migrations are never run automatically against production.
- Sample users all end in `@sample.invalid` (for example `superadmin@sample.invalid`) and share `SEED_PASSWORD`, or `sample-password-only` when it is unset. Sign-in arrives in Stage 4.
- Database tests create a throwaway database per test (`internal/dbtest`), so they never touch your local data.
- API database work runs inside `db.InScope(ctx, scope, fn)`: the transaction switches to the non-owner role `mezai_app` with the caller's scope (staff, org or contributor), and row-level security does the rest.
