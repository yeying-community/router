#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
deploy_dir="${ROUTER_DEPLOY_DIR:-/opt/deploy/router}"
build_dir="$repo_dir/build"
build_binary="$build_dir/router"
web_dir="$repo_dir/web"
target_binary="$deploy_dir/build/router"
starter="$deploy_dir/scripts/starter.sh"
pid_file="$deploy_dir/run/router.pid"
backup_binary=""
service_stopped=false
build_web="${ROUTER_BUILD_WEB:-1}"

log() {
  printf '[deploy] %s\n' "$*"
}

die() {
  printf '[deploy] ERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'USAGE'
Usage: scripts/deploy.sh [--skip-web]

Builds the embedded web frontend by default, then builds and deploys router.

Options:
  --skip-web      Skip npm install/build and use the existing web/dist assets.

Environment:
  ROUTER_BUILD_WEB=0              Skip frontend build.
  ROUTER_DEPLOY_DIR=/opt/deploy/router
  ROUTER_STARTUP_WAIT_SECONDS=3
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-web)
      build_web=0
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die "unknown argument: $1"
      ;;
  esac
done

rollback() {
  local status=$?

  if [[ "$service_stopped" == true ]]; then
    printf '[deploy] Deployment failed, attempting rollback.\n' >&2
    if [[ -n "$backup_binary" && -f "$backup_binary" ]]; then
      cp "$backup_binary" "$target_binary"
      chmod 0755 "$target_binary"
      printf '[deploy] Restored %s\n' "$backup_binary" >&2
    fi
    "$starter" start || true
  fi

  exit "$status"
}

trap rollback ERR

[[ -x "$starter" ]] || die "starter script not found or not executable: $starter"
command -v go >/dev/null 2>&1 || die "go command not found"

if [[ "$build_web" != "0" && "$build_web" != "false" && "$build_web" != "no" ]]; then
  [[ -d "$web_dir" ]] || die "web directory not found: $web_dir"
  [[ -f "$web_dir/package.json" ]] || die "web package.json not found: $web_dir/package.json"
  command -v npm >/dev/null 2>&1 || die "npm command not found"

  log "Building web frontend from $web_dir"
  (
    cd "$web_dir"
    if [[ -f package-lock.json ]]; then
      npm ci
    else
      npm install
    fi
    npm run build
  )
else
  log "Skipping web frontend build"
fi

[[ -f "$web_dir/dist/index.html" ]] || die "web dist index not found: $web_dir/dist/index.html"

log "Building router from $repo_dir"
mkdir -p "$build_dir"
(
  cd "$repo_dir"
  go build -o "$build_binary" ./cmd/router
)
[[ -x "$build_binary" ]] || die "build output is not executable: $build_binary"

if command -v sha256sum >/dev/null 2>&1; then
  log "Built binary: $(sha256sum "$build_binary" | awk '{print $1}')"
fi

log "Stopping router"
"$starter" stop
service_stopped=true

mkdir -p "$(dirname "$target_binary")"
if [[ -f "$target_binary" ]]; then
  backup_binary="${target_binary}.bak.$(date +%Y%m%d%H%M%S)"
  cp "$target_binary" "$backup_binary"
  log "Backed up current binary to $backup_binary"
fi

log "Replacing production binary"
install -m 0755 "$build_binary" "${target_binary}.new"
mv -f "${target_binary}.new" "$target_binary"

log "Starting router"
"$starter" start
sleep "${ROUTER_STARTUP_WAIT_SECONDS:-3}"

[[ -f "$pid_file" ]] || die "router pid file was not created: $pid_file"
pid="$(cat "$pid_file")"
[[ -n "$pid" ]] || die "router pid file is empty: $pid_file"
kill -0 "$pid" 2>/dev/null || die "router process is not running: pid $pid"

service_stopped=false
trap - ERR

log "Router deployment completed successfully, pid $pid"
if command -v sha256sum >/dev/null 2>&1; then
  log "Deployed binary: $(sha256sum "$target_binary" | awk '{print $1}')"
fi
