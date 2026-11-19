#!/usr/bin/env bash
# Staging chores run from a developer machine (Stage 19a). Settings come from
# backend/.env.staging (git-ignored; template in .env.staging.example).
#
#   scripts/staging.sh migrate        apply pending migrations
#   scripts/staging.sh status         show migration status
#   scripts/staging.sh check          prove Supabase's API roles have no access
#   scripts/staging.sh setup          create the first Super Admin (SETUP_ADMIN_*)
#   scripts/staging.sh seed           load sample data (needs SEED_PASSWORD)
#   scripts/staging.sh backup         pg_dump the public schema to $BACKUP_DIR
#   scripts/staging.sh restore-test   restore the newest backup into a throwaway
#                                     PostgreSQL and compare row counts
#   scripts/staging.sh demo [args]    run the pilot demo script (brief 18) in a
#                                     browser against DEMO_BASE_URL (default
#                                     https://mezai.vercel.app); args go to
#                                     Playwright, e.g. -g "[234]\. " or --headed
#
# Never point this file at production: production migrations are run by hand
# with the command in the PR notes (CLAUDE.md section 2).
set -euo pipefail

cd "$(dirname "$0")/.."
ENV_FILE=${STAGING_ENV_FILE:-./.env.staging}
[[ -f $ENV_FILE ]] || { echo "missing backend/$ENV_FILE (copy .env.staging.example)"; exit 1; }
# Read NAME=value lines without running the file as shell, so values may hold
# spaces and symbols. Matching quotes around a value are removed.
while IFS= read -r line || [[ -n $line ]]; do
  line=${line%$'\r'}
  [[ $line =~ ^[[:space:]]*(#|$) ]] && continue
  [[ $line =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=(.*)$ ]] || { echo "$ENV_FILE: cannot read a line (expected NAME=value)"; exit 1; }
  key=${BASH_REMATCH[1]}
  val=${BASH_REMATCH[2]}
  val=${val#"${val%%[![:space:]]*}"}
  val=${val%"${val##*[![:space:]]}"}
  if [[ ${#val} -ge 2 && ( ( ${val:0:1} == "'" && ${val: -1} == "'" ) || ( ${val:0:1} == '"' && ${val: -1} == '"' ) ) ]]; then
    val=${val:1:${#val}-2}
  fi
  export "$key=$val"
done <"$ENV_FILE"

: "${DATABASE_URL:?DATABASE_URL must be set in $ENV_FILE}"
# Containers reach the database by this URL. STAGING_ALLOW_LOCAL=1 lets the
# script be tested against the local Compose PostgreSQL.
DOCKER_DB_URL=$DATABASE_URL
case $DATABASE_URL in
  *localhost* | *127.0.0.1*)
    [[ ${STAGING_ALLOW_LOCAL:-} == 1 ]] || { echo "DATABASE_URL in $ENV_FILE points at this machine, not staging"; exit 1; }
    DOCKER_DB_URL=${DATABASE_URL/localhost/host.docker.internal}
    DOCKER_DB_URL=${DOCKER_DB_URL/127.0.0.1/host.docker.internal}
    ;;
esac

# psql and pg_dump refuse pgx-only options such as pool_max_conns.
DOCKER_DB_URL=$(sed -E 's/([?&])pool_[a-z_]+=[^&]*&?/\1/; s/[?&]$//' <<<"$DOCKER_DB_URL")

GOOSE="go run github.com/pressly/goose/v3/cmd/goose@v3.29.0"
PG_IMAGE=${PG_IMAGE:-postgres:17-alpine} # pg_dump must be at least the server's major version
BACKUP_DIR=${BACKUP_DIR:-$HOME/mezai-backups}
# Tables whose row counts a restore must reproduce.
CHECK_TABLES="users organisations contributor_profiles projects items submissions reviews earnings_ledger payout_batches audit_log goose_db_version"

counts() { # $1 = database URL; prints "table count" lines
  local sql="" t
  for t in $CHECK_TABLES; do sql+="SELECT '$t', count(*) FROM $t UNION ALL "; done
  docker run --rm "$PG_IMAGE" psql "$1" -At -F ' ' -c "${sql% UNION ALL } ORDER BY 1"
}

case ${1:-} in
  migrate) $GOOSE -dir migrations postgres "$DATABASE_URL" up ;;
  status) $GOOSE -dir migrations postgres "$DATABASE_URL" status ;;
  check) # Supabase's API roles must hold no privileges on our tables (00028)
    docker run --rm "$PG_IMAGE" psql "$DOCKER_DB_URL" -At -F ': ' -c "
      SELECT 'anon/authenticated table grants (want 0)', count(*) FROM information_schema.role_table_grants
        WHERE grantee IN ('anon','authenticated') AND table_schema = 'public'
      UNION ALL SELECT 'anon/authenticated default privileges from our role in public (want 0)', count(*)
        FROM pg_default_acl d JOIN pg_namespace n ON n.oid = d.defaclnamespace
        WHERE n.nspname = 'public' AND d.defaclrole = current_user::regrole
          AND array_to_string(d.defaclacl, ',') ~ '(^|,)(anon|authenticated)='
      UNION ALL SELECT 'mezai_app table grants (want > 0)', count(*) FROM information_schema.role_table_grants
        WHERE grantee = 'mezai_app'"
    echo "Other default privileges naming those roles (other owners or schemas; not ours to change):"
    docker run --rm "$PG_IMAGE" psql "$DOCKER_DB_URL" -At -F ' | ' -c "
      SELECT d.defaclrole::regrole, coalesce(n.nspname, '(all schemas)'), d.defaclobjtype
      FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid = d.defaclnamespace
      WHERE array_to_string(d.defaclacl, ',') ~ '(^|,)(anon|authenticated)='
        AND NOT (n.nspname = 'public' AND d.defaclrole = current_user::regrole) ORDER BY 1, 2, 3"
    ;;
  setup) go run ./cmd/setup ;;
  seed)
    # The repository is public, so the built-in sample password is too.
    [[ ${#SEED_PASSWORD} -ge 16 ]] || { echo "set SEED_PASSWORD (16+ characters) in $ENV_FILE"; exit 1; }
    # One transaction over the internet: allow more than the local minute.
    APP_ENV=development SEED_TIMEOUT=${SEED_TIMEOUT:-10m} go run ./cmd/seed
    ;;
  backup)
    mkdir -p "$BACKUP_DIR"
    out="$BACKUP_DIR/mezai-staging-$(date -u +%Y%m%dT%H%M%SZ).dump"
    docker run --rm "$PG_IMAGE" pg_dump "$DOCKER_DB_URL" --schema=public --format=custom >"$out"
    echo "backup: $out ($(du -h "$out" | cut -f1))"
    ;;
  restore-test)
    file=$(ls -t "$BACKUP_DIR"/mezai-staging-*.dump 2>/dev/null | head -1)
    [[ -n $file ]] || { echo "no backup in $BACKUP_DIR; run backup first"; exit 1; }
    name=mezai-restore-test-$$
    trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
    docker run -d --name "$name" -e POSTGRES_PASSWORD=restore_test_only -v "$file:/backup.dump:ro" "$PG_IMAGE" >/dev/null
    until docker exec "$name" pg_isready -U postgres -q; do sleep 1; done
    sleep 2
    docker exec "$name" psql -U postgres -q -c "DROP SCHEMA public CASCADE" -c "CREATE ROLE mezai_app NOLOGIN"
    # --no-acl: Supabase's own roles do not exist here. Grants are not part of
    # what this test proves; a restore into Supabase keeps them. Supabase adds a
    # per-column option (pg_attver) stock PostgreSQL rejects, so those lines are
    # dropped. ON_ERROR_STOP makes any other problem fail the test.
    docker exec "$name" sh -c "pg_restore --no-owner --no-acl -f - /backup.dump | grep -v 'SET (pg_attver=' | psql -U postgres -d postgres -q -v ON_ERROR_STOP=1 >/dev/null"
    docker exec "$name" psql -U postgres -At -F ' ' -c "$(for t in $CHECK_TABLES; do printf "SELECT '%s', count(*) FROM %s UNION ALL " "$t" "$t"; done | sed 's/ UNION ALL $//') ORDER BY 1" >"$BACKUP_DIR/restored.txt"
    counts "$DOCKER_DB_URL" >"$BACKUP_DIR/live.txt"
    echo "restore-test: $file"
    if diff -u "$BACKUP_DIR/live.txt" "$BACKUP_DIR/restored.txt"; then
      echo "restore-test: row counts match ($(wc -l <"$BACKUP_DIR/live.txt" | tr -d ' ') tables)"
    else
      echo "restore-test: counts differ (writes since the backup show up here too; rerun right after a backup)"
      exit 1
    fi
    ;;
  demo)
    shift
    [[ ${#SEED_PASSWORD} -ge 16 ]] || { echo "set SEED_PASSWORD (the staging seed's) in $ENV_FILE"; exit 1; }
    # The runner signs in as the seeded staff and contributors and uses psql
    # (in Docker) for its few database steps (frontend/tests/demo/db.ts).
    cd ../frontend
    DEMO_BASE_URL=${DEMO_BASE_URL:-https://mezai.vercel.app} DEMO_DB_URL=$DOCKER_DB_URL PG_IMAGE=$PG_IMAGE \
      SEED_PASSWORD=$SEED_PASSWORD npx playwright test -c playwright.demo.config.ts "$@"
    ;;
  *) sed -n '2,18p' "$0"; exit 1 ;;
esac
