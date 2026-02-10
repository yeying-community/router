#!/usr/bin/env bash
set -euo pipefail

cmd="${1:-start}"
root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin_path="$root_dir/build/router"
log_dir="${ROUTER_LOG_DIR:-$root_dir/logs}"
pid_dir="$root_dir/run"
pid_file="$pid_dir/router.pid"
port="${ROUTER_PORT:-3011}"

ensure_dirs() {
  mkdir -p "$log_dir" "$pid_dir"
}

is_running() {
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file")"
    if [[ -n "$pid" ]] && kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
  fi
  return 1
}

start() {
  ensure_dirs
  if [[ ! -x "$bin_path" ]]; then
    echo "Binary not found or not executable: $bin_path" >&2
    echo "Build first: mkdir -p build && go build -o build/router ./cmd/router" >&2
    exit 1
  fi

  if is_running; then
    echo "Router already running (pid $(cat "$pid_file"))."
    return 0
  fi

  cd "$root_dir"
  nohup "$bin_path" --port "$port" --log-dir "$log_dir" \
    >>"$log_dir/starter.log" 2>&1 &
  echo $! > "$pid_file"
  echo "Router started (pid $(cat "$pid_file")), port $port."
}

stop() {
  if [[ ! -f "$pid_file" ]]; then
    echo "Router not running (pid file missing)."
    return 0
  fi

  local pid
  pid="$(cat "$pid_file")"
  if [[ -z "$pid" ]] || ! kill -0 "$pid" 2>/dev/null; then
    rm -f "$pid_file"
    echo "Router not running."
    return 0
  fi

  kill "$pid"
  for _ in {1..50}; do
    if ! kill -0 "$pid" 2>/dev/null; then
      break
    fi
    sleep 0.2
  done

  if kill -0 "$pid" 2>/dev/null; then
    kill -9 "$pid" 2>/dev/null || true
  fi

  rm -f "$pid_file"
  echo "Router stopped."
}

restart() {
  stop
  start
}

case "$cmd" in
  start) start ;;
  stop) stop ;;
  restart) restart ;;
  *)
    echo "Usage: $(basename "$0") {start|stop|restart}" >&2
    exit 1
    ;;
esac
