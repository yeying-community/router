#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
deploy_dir="${ROUTER_DEPLOY_DIR:-/opt/deploy/router}"
build_dir="$repo_dir/build"
build_binary="$build_dir/router"
web_dir="$repo_dir/web"
target_binary="$deploy_dir/build/router"
target_version_file="$deploy_dir/VERSION"
starter="$deploy_dir/scripts/starter.sh"
pid_file="$deploy_dir/run/router.pid"
backup_binary=""
backup_version_file=""
service_stopped=false
version_file_updated=false
build_web="${ROUTER_BUILD_WEB:-1}"
router_version="${ROUTER_VERSION:-$(git -C "$repo_dir" describe --tags --always --dirty 2>/dev/null || git -C "$repo_dir" rev-parse --short HEAD)}"

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
  ROUTER_VERSION=<release version> Override the embedded and reported version.
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
    if [[ "$version_file_updated" == true ]]; then
      if [[ -n "$backup_version_file" && -f "$backup_version_file" ]]; then
        cp "$backup_version_file" "$target_version_file"
      else
        rm -f "$target_version_file"
      fi
    fi
    rm -f "${target_version_file}.new"
    "$starter" start || true
  fi

  exit "$status"
}

trap rollback ERR

[[ "$router_version" =~ ^[[:alnum:]._+-]+$ ]] || die "invalid router version: $router_version"
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
  go build -ldflags "-X github.com/yeying-community/router/common.Version=$router_version" -o "$build_binary" ./cmd/router
)
[[ -x "$build_binary" ]] || die "build output is not executable: $build_binary"
embedded_version="$("$build_binary" --version 2>/dev/null | head -n 1)"
[[ "$embedded_version" == "$router_version" ]] || die "built version mismatch: expected $router_version, got ${embedded_version:-unknown}"

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
if [[ -f "$target_version_file" ]]; then
  backup_version_file="${target_version_file}.bak.$(date +%Y%m%d%H%M%S)"
  cp "$target_version_file" "$backup_version_file"
fi

log "Replacing production binary"
install -m 0755 "$build_binary" "${target_binary}.new"
mv -f "${target_binary}.new" "$target_binary"
printf '%s\n' "$router_version" > "${target_version_file}.new"
mv -f "${target_version_file}.new" "$target_version_file"
version_file_updated=true

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
