#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_PATH="$(realpath "$SCRIPT_DIR/..")"
MODULE_NAME="$(basename "$REAL_PATH")"
CONF_FILE="$SCRIPT_DIR/backup.conf"
CONFIG_FILE="$REAL_PATH/config.yaml"
BACKUP_DIR="/opt/backup"
TMP_BASE_DIR="/tmp"
LOGFILE=""
EXIT_BACKUP_SUCCESS=0
EXIT_BACKUP_FAILED=1
# Shell exit codes are unsigned 8-bit values; use 255 as the portable form of -1.
EXIT_BACKUP_EXISTS=255

BACKUP_CONF_FLAG="False"
BACKUP_CONF_PREFIX=""
BACKUP_CONF_SUFFIX="conf.tar.gz"

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

load_backup_conf() {
    if [[ ! -f "$CONF_FILE" ]]; then
        log_err "backup config not found: $CONF_FILE"
        exit 1
    fi

    # shellcheck source=/dev/null
    source "$CONF_FILE"
}

validate_backup_conf() {
    case "$BACKUP_CONF_FLAG" in
        True|False) ;;
        *)
            log_err "invalid BACKUP_CONF_FLAG: $BACKUP_CONF_FLAG; expected True or False"
            exit 1
            ;;
    esac

    if [[ -z "$BACKUP_CONF_SUFFIX" ]]; then
        log_err "BACKUP_CONF_SUFFIX must not be empty"
        exit 1
    fi
}

backup_config() {
    local backup_file_name="${BACKUP_CONF_PREFIX}${MODULE_NAME}${BACKUP_CONF_SUFFIX}"
    local backup_file_path="$BACKUP_DIR/$backup_file_name"
    local tmp_conf_dir="$TMP_BASE_DIR/${MODULE_NAME}-conf"

    if [[ "$BACKUP_CONF_FLAG" == "False" ]]; then
        log "config backup skipped: BACKUP_CONF_FLAG=False"
        return "$EXIT_BACKUP_SUCCESS"
    fi

    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_err "config file not found: $CONFIG_FILE"
        exit 1
    fi

    mkdir -p "$BACKUP_DIR"

    if [[ -f "$backup_file_path" ]]; then
        log "backup already exists, skip: $backup_file_path"
        return "$EXIT_BACKUP_EXISTS"
    fi

    rm -rf "$tmp_conf_dir"
    mkdir -p "$tmp_conf_dir"

    log "copy config file to temporary directory: $tmp_conf_dir"
    cp "$CONFIG_FILE" "$tmp_conf_dir/config.yaml"

    log "create backup file: $backup_file_path"
    (
        cd "$TMP_BASE_DIR"
        tar -czf "$backup_file_path" "${MODULE_NAME}-conf"
    )

    rm -rf "$tmp_conf_dir"
    log "config backup completed: $backup_file_path"
    return "$EXIT_BACKUP_SUCCESS"
}

cleanup() {
    rm -rf "$TMP_BASE_DIR/${MODULE_NAME}-conf"
}
trap cleanup EXIT

init_log_file "config-backup-router.log"
log "config backup started for $MODULE_NAME at $REAL_PATH"
load_backup_conf
validate_backup_conf
if backup_config; then
    exit "$EXIT_BACKUP_SUCCESS"
else
    status=$?
    if [[ "$status" -eq "$EXIT_BACKUP_EXISTS" ]]; then
        exit "$EXIT_BACKUP_EXISTS"
    fi

    exit "$EXIT_BACKUP_FAILED"
fi
