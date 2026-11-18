#!/usr/bin/env bash
# Staging chores run from a developer machine (Stage 19a). Settings come from
# backend/.env.staging (git-ignored; template in .env.staging.example).
#
#   scripts/staging.sh migrate        apply pending migrations
#   scripts/staging.sh status         show migration status
#   scripts/staging.sh setup          create the first Super Admin (SETUP_ADMIN_*)
#   scripts/staging.sh seed           load sample data (needs SEED_PASSWORD)
#   scripts/staging.sh backup         pg_dump the public schema to $BACKUP_DIR
#   scripts/staging.sh restore-test   restore the newest backup into a throwaway
#                                     PostgreSQL and compare row counts
#
# Never point this file at production: production migrations are run by hand
# with the command in the PR notes (CLAUDE.md section 2).
set -euo pipefail

cd "$(dirname "$0")/.."
ENV_FILE=${STAGING_ENV_FILE:-./.env.staging}
[[ -f $ENV_FILE ]] || { echo "missing backend/$ENV_FILE (copy .env.staging.example)"; exit 1; }
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

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
  setup) go run ./cmd/setup ;;
  seed)
    # The repository is public, so the built-in sample password is too.
    [[ ${#SEED_PASSWORD} -ge 16 ]] || { echo "set SEED_PASSWORD (16+ characters) in $ENV_FILE"; exit 1; }
    APP_ENV=development go run ./cmd/seed
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
    # what this test proves; a restore into Supabase keeps them.
    docker exec "$name" pg_restore -U postgres -d postgres --no-owner --no-acl /backup.dump
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
  *) sed -n '2,13p' "$0"; exit 1 ;;
esac
