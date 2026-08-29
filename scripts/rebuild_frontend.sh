#!/usr/bin/env bash
# Rebuild frontend dist and force-recreate the frontend container.
#
# Use this after editing anything under frontend/src/ so the running
# nginx-backed frontend picks up the change. Plain `vite build` only
# rewrites frontend/dist on the host — the WeKnora-frontend container
# still serves the dist that was COPY'd at image build time, so the
# browser will keep showing stale bundles until the container is
# recreated.
#
# Usage:
#   ./scripts/rebuild_frontend.sh
#
# Skip the docker step (CI / dist-only packaging) with --no-recreate.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

RECREATE=1
for arg in "$@"; do
	case "$arg" in
	--no-recreate) RECREATE=0 ;;
	-h|--help)
		sed -n '2,15p' "$0"
		exit 0
		;;
	*)
		printf 'unknown arg: %s\n' "$arg" >&2
		exit 2
		;;
	esac
done

"$PROJECT_ROOT/scripts/build_frontend_dist.sh"

if [ "$RECREATE" -eq 1 ]; then
	if ! command -v docker >/dev/null 2>&1; then
		printf 'docker not found; skipping container recreate.\n' >&2
		printf 'run \"./scripts/rebuild_frontend.sh --no-recreate\" or recreate the frontend container manually.\n'
		exit 0
	fi
	if ! docker compose version >/dev/null 2>&1; then
		printf 'docker compose plugin not found; skipping container recreate.\n' >&2
		exit 0
	fi

	cd "$PROJECT_ROOT"
	# WeKnora 的 frontend Dockerfile 是 `COPY dist /usr/share/nginx/html`，
	# 镜像在 build 时把 dist 烤进去。仅 recreate 容器不会重新 build image，
	# 浏览器仍会拿到老 bundle，所以必须先 build 再 up。
	docker compose build frontend
	docker compose up -d --force-recreate --no-deps frontend
	printf 'frontend image rebuilt and container recreated; refresh the browser with Ctrl+Shift+R.\n'
fi