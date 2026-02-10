#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project_name="${PROJECT_NAME:-$(basename "$root_dir")}"
timestamp="$(date +%Y%m%d-%H%M%S)"
git_hash="$(git -C "$root_dir" rev-parse --short=7 HEAD 2>/dev/null || echo "unknown")"
pkg_name="${project_name}-${timestamp}-${git_hash}"

out_dir="$root_dir/output"
stage_dir="$out_dir/$pkg_name"
bin_src="$root_dir/build/router"
env_template="$root_dir/.env.template"
web_build_dir="$root_dir/web/dist"
starter_src="$root_dir/scripts/starter.sh"

if [[ ! -x "$bin_src" ]]; then
  echo "Missing binary: $bin_src" >&2
  echo "Build first: mkdir -p build && go build -o build/router ./cmd/router" >&2
  exit 1
fi

if [[ ! -d "$web_build_dir" ]]; then
  echo "Missing frontend build: $web_build_dir" >&2
  echo "Build first: npm run build --prefix web" >&2
  exit 1
fi

if [[ ! -f "$env_template" ]]; then
  echo "Missing config template: $env_template" >&2
  exit 1
fi

if [[ ! -f "$starter_src" ]]; then
  echo "Missing starter script: $starter_src" >&2
  exit 1
fi

rm -rf "$stage_dir"
mkdir -p "$stage_dir/build" "$stage_dir/scripts" "$stage_dir/web"

cp "$bin_src" "$stage_dir/build/"
cp "$env_template" "$stage_dir/"
cp "$starter_src" "$stage_dir/scripts/"
cp -R "$web_build_dir" "$stage_dir/web/"

mkdir -p "$out_dir"
tar -czf "$out_dir/$pkg_name.tar.gz" -C "$out_dir" "$pkg_name"

if [[ "${KEEP_STAGE:-0}" != "1" ]]; then
  rm -rf "$stage_dir"
fi

echo "Package created: $out_dir/$pkg_name.tar.gz"
