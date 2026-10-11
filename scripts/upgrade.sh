#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 3 ]]; then
  printf 'Usage: %s <prepare|post-copy|finalize> <current-dir> <target-dir>\n' "$0" >&2
  exit 1
fi

case "$1" in
  prepare|post-copy|finalize)
    exit 0
    ;;
  *)
    printf 'Unsupported upgrade phase: %s\n' "$1" >&2
    exit 1
    ;;
esac
