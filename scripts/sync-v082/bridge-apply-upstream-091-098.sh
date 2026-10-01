#!/usr/bin/env bash
# Bridge: apply upstream v0.8.2 migrations 091-098 onto a 川农库 that already
# recorded schema_migrations.version = 98 with *teaching* SQL under those numbers.
#
# After this script succeeds, leave version at 98 and run normal migrate up
# (app AUTO_MIGRATE or scripts/migrate.sh up) to apply 099-110 then 111-116.
#
# SAFETY
# - Only run against a CLONED volume / disposable DB.
# - Never point this at the production / source docker volume.
# - Requires CONFIRM=YES.
# - Idempotent via ledger table sicau_v082_bridge_ledger.
#
# Rollback if this fails mid-way: discard the clone (preferred). Do not force
# schema_migrations on the source volume. See docs/上游同步-v0.8.2-迁移与回滚.md.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
MIGRATIONS_DIR="${MIGRATIONS_DIR:-$REPO_ROOT/migrations/versioned}"

CONFIRM="${CONFIRM:-}"
DB_URL="${DB_URL:-}"
PSQL_CMD="${PSQL_CMD:-}" # optional: full psql invocation prefix, e.g. docker exec -i WeKnora-postgres psql -U postgres -d WeKnora

die() { echo "ERROR: $*" >&2; exit 1; }

if [[ "$CONFIRM" != "YES" ]]; then
  die "refusing to run without CONFIRM=YES (clone-only bridge)"
fi

if [[ -z "$DB_URL" && -z "$PSQL_CMD" ]]; then
  die "set DB_URL=postgres://... or PSQL_CMD='docker exec -i <pg> psql -U ... -d ...'"
fi

run_psql() {
  if [[ -n "$PSQL_CMD" ]]; then
    # shellcheck disable=SC2086
    eval "$PSQL_CMD" "$@"
  else
    psql "$DB_URL" "$@"
  fi
}

sql_scalar() {
  run_psql -v ON_ERROR_STOP=1 -Atc "$1"
}

echo "==> preflight: schema_migrations version"
VERSION="$(sql_scalar "SELECT version FROM schema_migrations LIMIT 1;" || true)"
DIRTY="$(sql_scalar "SELECT dirty::text FROM schema_migrations LIMIT 1;" || true)"
echo "    version=${VERSION:-<none>} dirty=${DIRTY:-<none>}"

if [[ -z "${VERSION:-}" ]]; then
  die "no schema_migrations row — use empty-DB path (migrate up to 116), not this bridge"
fi
if [[ "$DIRTY" == "t" || "$DIRTY" == "true" ]]; then
  die "schema_migrations.dirty=true — resolve dirty state on the CLONE before bridging"
fi
if [[ "$VERSION" != "98" ]]; then
  die "expected schema_migrations.version=98 (川农线), got $VERSION — abort"
fi

echo "==> ensure ledger"
run_psql -v ON_ERROR_STOP=1 <<'SQL'
CREATE TABLE IF NOT EXISTS sicau_v082_bridge_ledger (
    step        TEXT PRIMARY KEY,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note        TEXT NOT NULL DEFAULT ''
);
SQL

ALREADY="$(sql_scalar "SELECT COUNT(*) FROM sicau_v082_bridge_ledger WHERE step = 'complete';")"
if [[ "$ALREADY" == "1" ]]; then
  echo "==> bridge already complete — nothing to do"
  exit 0
fi

STEPS=(
  "000091_mcp_tool_enabled"
  "000092_mcp_metadata"
  "000093_browser_authorization"
  "000094_memory_consistency"
  "000095_memory_vector_search"
  "000096_dingtalk_stream_only"
  "000097_session_fork"
  "000098_fork_snapshot_lease"
)

for step in "${STEPS[@]}"; do
  DONE="$(sql_scalar "SELECT COUNT(*) FROM sicau_v082_bridge_ledger WHERE step = '$step';")"
  if [[ "$DONE" == "1" ]]; then
    echo "==> skip $step (ledger)"
    continue
  fi
  FILE="$MIGRATIONS_DIR/${step}.up.sql"
  [[ -f "$FILE" ]] || die "missing $FILE"
  echo "==> apply $step"
  if ! run_psql -v ON_ERROR_STOP=1 -f "$FILE"; then
    die "failed applying $step — STOP. Discard the clone volume to roll back. Do not force version on source."
  fi
  run_psql -v ON_ERROR_STOP=1 -c \
    "INSERT INTO sicau_v082_bridge_ledger(step, note) VALUES ('$step', 'upstream v0.8.2 SQL applied without bumping schema_migrations');"
done

run_psql -v ON_ERROR_STOP=1 -c \
  "INSERT INTO sicau_v082_bridge_ledger(step, note) VALUES ('complete', 'ready for migrate up 099-116') ON CONFLICT (step) DO NOTHING;"

AFTER="$(sql_scalar "SELECT version FROM schema_migrations LIMIT 1;")"
echo "==> bridge OK; schema_migrations.version still $AFTER (expect 98)"
echo "==> next: migrate up (AUTO_MIGRATE or scripts/migrate.sh up) → 099..116"
echo "==> rollback if later steps fail: discard clone volume; never down -v the source volume"
