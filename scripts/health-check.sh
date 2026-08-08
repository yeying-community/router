#!/usr/bin/env bash
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

PROJECT_NAME="router"
SCHEMA_VERSION="1.0"

LEVEL="${HEALTH_LEVEL:-readiness}"
TIMEOUT="${HEALTH_TIMEOUT:-10}"
RETRIES="${HEALTH_RETRIES:-0}"
INTERVAL="${HEALTH_INTERVAL:-2}"
FORMAT="${HEALTH_FORMAT:-text}"
CONFIG="${HEALTH_CONFIG:-$PROJECT_DIR/config.yaml}"
BASE_URL="${HEALTH_BASE_URL:-}"
LOGFILE=""
WAIT_SECONDS=0
QUIET=false
VERBOSE=false
COMPONENTS=()

CHECK_NAMES=()
CHECK_STATUSES=()
CHECK_DURATIONS=()
CHECK_MESSAGES=()
CHECK_STATUS=""
CHECK_MESSAGE=""
WAIT_TIMED_OUT=false
STARTED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
RUN_STARTED_MS=0
RUN_DURATION_MS=0

init_log_file() {
  local logfile_name=$1
  local logfile_dir="/opt/logs"

  LOGFILE="${logfile_dir}/${logfile_name}"
  mkdir -p "$logfile_dir"
  touch "$LOGFILE"

  local filesize=0
  filesize=$(stat -c "%s" "$LOGFILE" 2>/dev/null || echo 0)
  if [[ "$filesize" -ge 1048576 ]]; then
    printf 'clear old logs at %s to avoid log file too big\n' "$(date)" > "$LOGFILE"
  fi
}

log() {
  echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOGFILE"
}

log_err() {
  echo -e "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOGFILE" >&2
}

log_info() {
  log "$@" >/dev/null
}

log_error_info() {
  log_err "$@" >/dev/null 2>&1
}

usage() {
  cat <<'USAGE'
Usage: scripts/health-check.sh [options]

Options:
  --level <level>       liveness, readiness, dependency, or all (default: readiness)
  --timeout <seconds>   Per-check timeout in seconds (default: 10)
  --retries <count>     Retries after the first failed attempt (default: 0)
  --interval <seconds>  Retry/wait interval in seconds (default: 2)
  --format <format>     text or json (default: text)
  --config <path>       Router config path (default: ./config.yaml)
  --base-url <url>      Router base URL (default: http://127.0.0.1:<port>)
  --component <name>    Only run one check component; repeatable
  --wait <seconds>      Wait up to this many seconds for checks to pass
  --quiet               Only print final result plus warnings/failures in text mode
  --verbose             Include extra diagnostic messages where safe
  --help                Show this help

Environment:
  HEALTH_BASE_URL, HEALTH_TIMEOUT, HEALTH_RETRIES, HEALTH_INTERVAL,
  HEALTH_FORMAT, HEALTH_CONFIG, ROUTER_PORT
USAGE
}

die_usage() {
  log_error_info "usage error: $1"
  printf 'health-check: %s\n' "$1" >&2
  exit 2
}

die_framework() {
  log_error_info "framework error: $1"
  printf 'health-check: %s\n' "$1" >&2
  exit 3
}

is_non_negative_int() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

is_positive_int() {
  [[ "$1" =~ ^[1-9][0-9]*$ ]]
}

need_value() {
  local opt="$1"
  local value="${2:-}"
  if [[ -z "$value" || "$value" == --* ]]; then
    die_usage "$opt requires a value"
  fi
}

init_log_file "health-check-router.log"
log_info "health check invoked: level=$LEVEL format=$FORMAT config=$CONFIG base_url=${BASE_URL:-<auto>} timeout=$TIMEOUT retries=$RETRIES interval=$INTERVAL wait=$WAIT_SECONDS"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --level)
      need_value "$1" "${2:-}"
      LEVEL="$2"
      shift 2
      ;;
    --timeout)
      need_value "$1" "${2:-}"
      TIMEOUT="$2"
      shift 2
      ;;
    --retries)
      need_value "$1" "${2:-}"
      RETRIES="$2"
      shift 2
      ;;
    --interval)
      need_value "$1" "${2:-}"
      INTERVAL="$2"
      shift 2
      ;;
    --format)
      need_value "$1" "${2:-}"
      FORMAT="$2"
      shift 2
      ;;
    --config)
      need_value "$1" "${2:-}"
      CONFIG="$2"
      shift 2
      ;;
    --base-url)
      need_value "$1" "${2:-}"
      BASE_URL="$2"
      shift 2
      ;;
    --component)
      need_value "$1" "${2:-}"
      COMPONENTS+=("$2")
      shift 2
      ;;
    --wait)
      need_value "$1" "${2:-}"
      WAIT_SECONDS="$2"
      shift 2
      ;;
    --quiet)
      QUIET=true
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      die_usage "unknown argument: $1"
      ;;
  esac
done

log_info "health check options parsed: level=$LEVEL format=$FORMAT config=$CONFIG base_url=${BASE_URL:-<auto>} timeout=$TIMEOUT retries=$RETRIES interval=$INTERVAL wait=$WAIT_SECONDS components=${COMPONENTS[*]:-<all>} quiet=$QUIET verbose=$VERBOSE"

case "$LEVEL" in
  liveness|readiness|dependency|all) ;;
  *) die_usage "invalid --level: $LEVEL" ;;
esac

case "$FORMAT" in
  text|json) ;;
  *) die_usage "invalid --format: $FORMAT" ;;
esac

is_positive_int "$TIMEOUT" || die_usage "--timeout must be a positive integer"
is_non_negative_int "$RETRIES" || die_usage "--retries must be a non-negative integer"
is_non_negative_int "$INTERVAL" || die_usage "--interval must be a non-negative integer"
is_non_negative_int "$WAIT_SECONDS" || die_usage "--wait must be a non-negative integer"

command -v curl >/dev/null 2>&1 || die_framework "curl command not found"

now_ms() {
  if command -v python3 >/dev/null 2>&1; then
    python3 -c 'import time; print(int(time.time() * 1000))'
  else
    printf '%s000\n' "$(date +%s)"
  fi
}

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

lower_status() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

yaml_value() {
  local section="$1"
  local key="$2"
  local file="$3"
  [[ -f "$file" ]] || return 1
  awk -v section="$section" -v key="$key" '
    function trim(v) {
      sub(/^[[:space:]]+/, "", v)
      sub(/[[:space:]]+$/, "", v)
      return v
    }
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    /^[^[:space:]#][^:]*:/ {
      current=$0
      sub(/:.*/, "", current)
      current=trim(current)
      next
    }
    current == section {
      line=$0
      sub(/^[[:space:]]+/, "", line)
      if (line ~ "^" key ":[[:space:]]*") {
        sub("^" key ":[[:space:]]*", "", line)
        sub(/[[:space:]]+#.*/, "", line)
        line=trim(line)
        if ((line ~ /^".*"$/) || (line ~ /^\047.*\047$/)) {
          line=substr(line, 2, length(line)-2)
        }
        print line
        found=1
        exit
      }
    }
    END { if (!found) exit 1 }
  ' "$file"
}

resolve_base_url() {
  if [[ -n "$BASE_URL" ]]; then
    BASE_URL="${BASE_URL%/}"
    return
  fi
  local port="${ROUTER_PORT:-}"
  if [[ -z "$port" ]]; then
    port="$(yaml_value server port "$CONFIG" 2>/dev/null || true)"
  fi
  if [[ -z "$port" ]]; then
    port="3011"
  fi
  BASE_URL="http://127.0.0.1:$port"
}

project_version() {
  local version_file="$PROJECT_DIR/VERSION"
  if [[ -s "$version_file" ]]; then
    head -n 1 "$version_file"
    return
  fi
  if [[ -x "$PROJECT_DIR/build/router" ]]; then
    "$PROJECT_DIR/build/router" --version 2>/dev/null | head -n 1
    return
  fi
  printf 'unknown'
}

component_enabled() {
  local name="$1"
  if [[ "${#COMPONENTS[@]}" -eq 0 ]]; then
    return 0
  fi
  local item
  for item in "${COMPONENTS[@]}"; do
    if [[ "$item" == "$name" ]]; then
      return 0
    fi
  done
  return 1
}

add_check() {
  CHECK_NAMES+=("$1")
  CHECK_STATUSES+=("$2")
  CHECK_DURATIONS+=("$3")
  CHECK_MESSAGES+=("$4")
}

set_check() {
  CHECK_STATUS="$1"
  CHECK_MESSAGE="$2"
}

run_check() {
  local name="$1"
  local fn="$2"
  shift 2
  component_enabled "$name" || return 0

  local attempt=0
  local start_ms end_ms duration_ms
  while :; do
    CHECK_STATUS=""
    CHECK_MESSAGE=""
    start_ms="$(now_ms)"
    "$fn" "$@" >/dev/null 2>&1
    end_ms="$(now_ms)"
    duration_ms=$((end_ms - start_ms))

    if [[ -z "$CHECK_STATUS" ]]; then
      CHECK_STATUS="FAIL"
      CHECK_MESSAGE="check did not set a result"
    fi

    if [[ "$CHECK_STATUS" != "FAIL" || "$attempt" -ge "$RETRIES" ]]; then
      add_check "$name" "$CHECK_STATUS" "$duration_ms" "$CHECK_MESSAGE"
      log_info "check completed: name=$name status=$CHECK_STATUS duration_ms=$duration_ms attempt=$attempt message=$CHECK_MESSAGE"
      return 0
    fi
    log_info "check retry: name=$name status=$CHECK_STATUS attempt=$attempt next_attempt=$((attempt + 1)) interval=$INTERVAL message=$CHECK_MESSAGE"
    attempt=$((attempt + 1))
    sleep "$INTERVAL"
  done
}

http_get() {
  local url="$1"
  local body_file error_file status
  body_file="$(mktemp "${TMPDIR:-/tmp}/router-health-body.XXXXXX")"
  error_file="$(mktemp "${TMPDIR:-/tmp}/router-health-error.XXXXXX")"
  status="$(curl -sS -L --max-time "$TIMEOUT" -o "$body_file" -w '%{http_code}' "$url" 2>"$error_file")"
  HTTP_STATUS="$status"
  HTTP_BODY="$(cat "$body_file")"
  HTTP_ERROR="$(cat "$error_file")"
  rm -f "$body_file" "$error_file"
  [[ "$status" =~ ^[0-9][0-9][0-9]$ && "$status" != "000" ]]
}

check_process() {
  local pid_file="$PROJECT_DIR/run/router.pid"
  if [[ ! -f "$pid_file" ]]; then
    set_check "SKIP" "pid file not found; service may be supervised externally"
    return 0
  fi
  local pid
  pid="$(cat "$pid_file" 2>/dev/null | tr -d '[:space:]')"
  if [[ -z "$pid" ]]; then
    set_check "FAIL" "pid file is empty"
    return 0
  fi
  if kill -0 "$pid" 2>/dev/null; then
    set_check "PASS" "router process is running"
  else
    set_check "FAIL" "pid file exists but process is not running"
  fi
}

check_http_liveness() {
  local url="$BASE_URL/api/v1/public/status"
  if http_get "$url"; then
    if [[ "$HTTP_STATUS" == "200" ]]; then
      set_check "PASS" "status endpoint is reachable"
    else
      set_check "FAIL" "status endpoint returned HTTP $HTTP_STATUS"
    fi
  else
    set_check "FAIL" "status endpoint is unreachable"
  fi
}

check_readiness_endpoint() {
  local url="$BASE_URL/api/v1/public/status"
  if ! http_get "$url"; then
    set_check "FAIL" "readiness endpoint is unreachable"
    return 0
  fi
  if [[ "$HTTP_STATUS" != "200" ]]; then
    set_check "FAIL" "readiness endpoint returned HTTP $HTTP_STATUS"
    return 0
  fi
  if printf '%s' "$HTTP_BODY" | grep -Eq '"success"[[:space:]]*:[[:space:]]*true'; then
    set_check "PASS" "readiness endpoint returned success=true"
  else
    set_check "FAIL" "readiness endpoint did not return success=true"
  fi
}

check_config() {
  if [[ ! -f "$CONFIG" ]]; then
    set_check "FAIL" "config file not found"
    return 0
  fi
  local dsn
  dsn="$(yaml_value database sql_dsn "$CONFIG" 2>/dev/null || true)"
  if [[ -z "$dsn" ]]; then
    set_check "FAIL" "database.sql_dsn is not configured"
    return 0
  fi
  set_check "PASS" "config file is readable and database.sql_dsn is set"
}

check_database() {
  if [[ ! -f "$CONFIG" ]]; then
    set_check "FAIL" "config file not found"
    return 0
  fi
  local dsn
  dsn="$(yaml_value database sql_dsn "$CONFIG" 2>/dev/null || true)"
  if [[ -z "$dsn" ]]; then
    set_check "FAIL" "database.sql_dsn is not configured"
    return 0
  fi
  if ! command -v psql >/dev/null 2>&1; then
    set_check "WARN" "psql is not installed; database dependency check skipped"
    return 0
  fi
  local output
  output="$(PGCONNECT_TIMEOUT="$TIMEOUT" psql "$dsn" -X -q -tA -c 'SELECT 1' 2>/dev/null || true)"
  if [[ "$output" == "1" ]]; then
    set_check "PASS" "database read-only query succeeded"
  else
    set_check "FAIL" "database read-only query failed"
  fi
}

check_redis() {
	local redis_conn cache_type
	redis_conn="$(yaml_value redis conn_string "$CONFIG" 2>/dev/null || true)"
	cache_type="$(yaml_value cache type "$CONFIG" 2>/dev/null || true)"
  if [[ -z "$redis_conn" && "$cache_type" != "redis" ]]; then
    set_check "SKIP" "redis is not configured"
    return 0
  fi
  if [[ -z "$redis_conn" ]]; then
    set_check "FAIL" "cache.type=redis but redis.conn_string is empty"
    return 0
  fi
	if ! command -v redis-cli >/dev/null 2>&1; then
		set_check "WARN" "redis-cli is not installed; redis dependency check skipped"
		return 0
	fi
	if [[ "$redis_conn" == *","* ]]; then
		set_check "WARN" "redis sentinel connection string is configured; redis dependency check skipped"
		return 0
	fi
	local scheme rest authority db auth hostport host port user password
	local -a tls_arg=()
	scheme="${redis_conn%%://*}"
	rest="${redis_conn#*://}"
	if [[ "$scheme" != "redis" && "$scheme" != "rediss" ]]; then
		set_check "WARN" "redis connection string scheme is unsupported by health check"
		return 0
	fi
	authority="${rest%%/*}"
	db="${rest#*/}"
	if [[ "$db" == "$rest" ]]; then
		db="0"
	fi
	db="${db%%\?*}"
	db="${db:-0}"
	if [[ "$authority" == *"@"* ]]; then
		auth="${authority%@*}"
		hostport="${authority#*@}"
		if [[ "$auth" == *":"* ]]; then
			user="${auth%%:*}"
			password="${auth#*:}"
		else
			user=""
			password="$auth"
		fi
	else
		hostport="$authority"
		user=""
		password=""
	fi
	host="${hostport%%:*}"
	port="${hostport##*:}"
	if [[ "$host" == "$port" ]]; then
		port="6379"
	fi
	if [[ -z "$host" ]]; then
		set_check "FAIL" "redis host is empty"
		return 0
	fi
	if [[ "$scheme" == "rediss" ]]; then
		tls_arg=(--tls)
	fi
	local output
	if [[ -n "$user" ]]; then
		output="$(REDISCLI_AUTH="$password" redis-cli "${tls_arg[@]}" --user "$user" -h "$host" -p "$port" -n "$db" --no-auth-warning ping 2>/dev/null || true)"
	elif [[ -n "$password" ]]; then
		output="$(REDISCLI_AUTH="$password" redis-cli "${tls_arg[@]}" -h "$host" -p "$port" -n "$db" --no-auth-warning ping 2>/dev/null || true)"
	else
		output="$(redis-cli "${tls_arg[@]}" -h "$host" -p "$port" -n "$db" --no-auth-warning ping 2>/dev/null || true)"
	fi
  if [[ "$output" == "PONG" ]]; then
    set_check "PASS" "redis PING succeeded"
  else
    set_check "FAIL" "redis PING failed"
  fi
}

check_billing_service() {
  local billing_base
  billing_base="$(yaml_value billing_service base_url "$CONFIG" 2>/dev/null || true)"
  if [[ -z "$billing_base" ]]; then
    set_check "SKIP" "billing service is not configured"
    return 0
  fi
  local url="${billing_base%/}/api/v1/internal/health"
  if http_get "$url"; then
    if [[ "$HTTP_STATUS" =~ ^2[0-9][0-9]$ ]]; then
      set_check "PASS" "billing service health endpoint returned HTTP $HTTP_STATUS"
    else
      set_check "WARN" "billing service health endpoint returned HTTP $HTTP_STATUS"
    fi
  else
    set_check "WARN" "billing service health endpoint is unreachable"
  fi
}

run_liveness_checks() {
  run_check "process" check_process
  run_check "http" check_http_liveness
}

run_readiness_checks() {
  run_liveness_checks
  run_check "readiness" check_readiness_endpoint
}

run_dependency_checks() {
  run_check "config" check_config
  run_check "database" check_database
  run_check "redis" check_redis
  run_check "billing_service" check_billing_service
}

run_selected_checks() {
  CHECK_NAMES=()
  CHECK_STATUSES=()
  CHECK_DURATIONS=()
  CHECK_MESSAGES=()
  RUN_STARTED_MS="$(now_ms)"
  STARTED_AT="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

  case "$LEVEL" in
    liveness)
      run_liveness_checks
      ;;
    readiness)
      run_readiness_checks
      ;;
    dependency)
      run_dependency_checks
      ;;
    all)
      run_liveness_checks
      run_check "readiness" check_readiness_endpoint
      run_dependency_checks
      ;;
  esac
  RUN_DURATION_MS=$(($(now_ms) - RUN_STARTED_MS))
}

summary_counts() {
  PASSED=0
  WARNED=0
  FAILED=0
  SKIPPED=0
  local status
  for status in "${CHECK_STATUSES[@]}"; do
    case "$status" in
      PASS) PASSED=$((PASSED + 1)) ;;
      WARN) WARNED=$((WARNED + 1)) ;;
      FAIL) FAILED=$((FAILED + 1)) ;;
      SKIP) SKIPPED=$((SKIPPED + 1)) ;;
    esac
  done
  OVERALL_STATUS="pass"
  if [[ "$FAILED" -gt 0 ]]; then
    OVERALL_STATUS="fail"
  elif [[ "$WARNED" -gt 0 ]]; then
    OVERALL_STATUS="warn"
  fi
}

output_text() {
  local i status
  for ((i = 0; i < ${#CHECK_NAMES[@]}; i++)); do
    status="${CHECK_STATUSES[$i]}"
    if [[ "$QUIET" == true && "$status" == "PASS" ]]; then
      continue
    fi
    if [[ "$QUIET" == true && "$status" == "SKIP" ]]; then
      continue
    fi
    printf '[%s] %s: %s (%s ms)\n' \
      "$status" "${CHECK_NAMES[$i]}" "${CHECK_MESSAGES[$i]}" "${CHECK_DURATIONS[$i]}"
  done
  printf 'RESULT status=%s passed=%d warned=%d failed=%d skipped=%d duration_ms=%d\n' \
    "$OVERALL_STATUS" "$PASSED" "$WARNED" "$FAILED" "$SKIPPED" "$RUN_DURATION_MS"
}

output_json() {
  local version i status comma
  version="$(project_version)"
  printf '{'
  printf '"schema_version":"%s",' "$SCHEMA_VERSION"
  printf '"type":"health_check",'
  printf '"project":"%s",' "$PROJECT_NAME"
  printf '"version":"%s",' "$(json_escape "$version")"
  printf '"environment":"%s",' "$(json_escape "${ROUTER_ENV:-}")"
  printf '"level":"%s",' "$LEVEL"
  printf '"status":"%s",' "$OVERALL_STATUS"
  printf '"started_at":"%s",' "$STARTED_AT"
  printf '"duration_ms":%d,' "$RUN_DURATION_MS"
  printf '"summary":{"passed":%d,"warned":%d,"failed":%d,"skipped":%d},' \
    "$PASSED" "$WARNED" "$FAILED" "$SKIPPED"
  printf '"checks":['
  comma=""
  for ((i = 0; i < ${#CHECK_NAMES[@]}; i++)); do
    status="$(lower_status "${CHECK_STATUSES[$i]}")"
    printf '%s{' "$comma"
    printf '"name":"%s",' "$(json_escape "${CHECK_NAMES[$i]}")"
    printf '"status":"%s",' "$status"
    printf '"duration_ms":%d,' "${CHECK_DURATIONS[$i]}"
    printf '"message":"%s"' "$(json_escape "${CHECK_MESSAGES[$i]}")"
    printf '}'
    comma=","
  done
  printf ']}'
  printf '\n'
}

resolve_base_url
log_info "health check started: project=$PROJECT_NAME level=$LEVEL config=$CONFIG base_url=$BASE_URL"

deadline_ms=$(($(now_ms) + WAIT_SECONDS * 1000))
while :; do
  run_selected_checks
  summary_counts
  if [[ "$FAILED" -eq 0 ]]; then
    break
  fi
  if [[ "$WAIT_SECONDS" -eq 0 ]]; then
    break
  fi
  if [[ "$(now_ms)" -ge "$deadline_ms" ]]; then
    WAIT_TIMED_OUT=true
    break
  fi
  sleep "$INTERVAL"
done

if [[ "$FORMAT" == "json" ]]; then
  output_json
else
  output_text
fi

if [[ "$FAILED" -gt 0 ]]; then
  if [[ "$WAIT_TIMED_OUT" == true ]]; then
    log_error_info "health check failed: status=$OVERALL_STATUS passed=$PASSED warned=$WARNED failed=$FAILED skipped=$SKIPPED duration_ms=$RUN_DURATION_MS exit_code=4 wait_timed_out=true"
    exit 4
  fi
  log_error_info "health check failed: status=$OVERALL_STATUS passed=$PASSED warned=$WARNED failed=$FAILED skipped=$SKIPPED duration_ms=$RUN_DURATION_MS exit_code=1"
  exit 1
fi
log_info "health check passed: status=$OVERALL_STATUS passed=$PASSED warned=$WARNED failed=$FAILED skipped=$SKIPPED duration_ms=$RUN_DURATION_MS exit_code=0"
exit 0
