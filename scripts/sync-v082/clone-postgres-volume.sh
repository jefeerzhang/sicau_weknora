#!/usr/bin/env bash
# Route 2 helper: clone postgres volume, optionally dump pre-bridge backup inside
# the clone, then print the exact bridge + migrate commands.
#
# Never writes to SOURCE_VOLUME. Destroying DEST_VOLUME is the rollback.

set -euo pipefail

SOURCE_VOLUME="${SOURCE_VOLUME:-weknora_postgres-data}"
DEST_VOLUME="${DEST_VOLUME:-weknora-sync-v082-upgrade_postgres-data}"
CONFIRM="${CONFIRM:-}"

die() { echo "ERROR: $*" >&2; exit 1; }

if [[ "$CONFIRM" != "YES" ]]; then
  die "refusing without CONFIRM=YES"
fi

if docker volume inspect "$SOURCE_VOLUME" >/dev/null 2>&1; then
  :
else
  die "source volume not found: $SOURCE_VOLUME"
fi

if docker volume inspect "$DEST_VOLUME" >/dev/null 2>&1; then
  die "dest volume already exists: $DEST_VOLUME — pick a new name or docker volume rm (CLONE ONLY)"
fi

echo "==> create $DEST_VOLUME"
docker volume create "$DEST_VOLUME" >/dev/null

echo "==> copy $SOURCE_VOLUME → $DEST_VOLUME (source stays read-only)"
docker run --rm \
  -v "$SOURCE_VOLUME:/from:ro" \
  -v "$DEST_VOLUME:/to" \
  alpine:3.20 \
  sh -c 'cp -a /from/. /to/'

echo "==> clone ready"
echo "    SOURCE (do not modify): $SOURCE_VOLUME"
echo "    CLONE (disposable):     $DEST_VOLUME"
echo
echo "Rollback = discard clone only:"
echo "  docker volume rm $DEST_VOLUME"
echo
echo "Next (example compose project weknora-sync-v082-upgrade):"
echo "  1. Point compose postgres volume at $DEST_VOLUME"
echo "  2. CONFIRM=YES PSQL_CMD='docker exec -i <pg-container> psql -U postgres -d WeKnora' \\"
echo "       bash scripts/sync-v082/bridge-apply-upstream-091-098.sh"
echo "  3. Start app with AUTO_MIGRATE=true (applies 099-116)"
echo "  4. If migrate fails: docker compose down (no -v on source); docker volume rm $DEST_VOLUME"
