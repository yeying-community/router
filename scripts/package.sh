#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
project_name="${PROJECT_NAME:-$(basename "$root_dir")}"
out_dir="$root_dir/output"
remote_name="${PACKAGE_REMOTE:-origin}"
auto_build="${AUTO_BUILD:-true}"
tag_arg="${1:-}"
source_dir=""
worktree_dir=""
bin_src=""
config_template=""
starter_src=""
health_check_src=""
backup_conf_template=""
config_backup_src=""
copy_for_upgrade_src=""
passphrase_template=""

usage() {
  echo "Usage: $(basename "$0") [v<major>.<minor>.<patch>]" >&2
}

is_semver_tag() {
  [[ "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

extract_max_tag() {
  git -C "$root_dir" tag -l 'v[0-9]*.[0-9]*.[0-9]*' | sort -V | tail -n 1
}

increment_patch_tag() {
  local tag="$1"
  local major minor patch
  major="${tag#v}"
  major="${major%%.*}"
  minor="${tag#v${major}.}"
  minor="${minor%%.*}"
  patch="${tag##*.}"
  echo "v${major}.${minor}.$((patch + 1))"
}

ensure_remote_exists() {
  if ! git -C "$root_dir" remote get-url "$remote_name" >/dev/null 2>&1; then
    echo "Remote not found: $remote_name" >&2
    exit 1
  fi
}

fetch_remote_refs() {
  ensure_remote_exists
  echo "Fetching latest refs from remote '$remote_name'..."
  git -C "$root_dir" fetch "$remote_name" --prune --tags
}

prepare_source_dir() {
  local ref="$1"
  worktree_dir="$(mktemp -d "${TMPDIR:-/tmp}/${project_name}-package.XXXXXX")"
  git -C "$root_dir" worktree add --detach "$worktree_dir" "$ref" >/dev/null
  source_dir="$worktree_dir"
  bin_src="$source_dir/build/router"
  config_template="$source_dir/config.yaml.template"
  starter_src="$source_dir/scripts/starter.sh"
  health_check_src="$source_dir/scripts/health-check.sh"
  backup_conf_template="$source_dir/scripts/backup.conf.template"
  config_backup_src="$source_dir/scripts/config_backup.sh"
  copy_for_upgrade_src="$source_dir/scripts/copy-for-upgrade.sh"
  upgrade_src="$source_dir/scripts/upgrade.sh"
  passphrase_template="$source_dir/scripts/.passphrase-file.template"
}

verify_artifacts() {
  local web_build_dir="$source_dir/web/dist"
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
  if [[ ! -f "$config_template" ]]; then
    echo "Missing config template: $config_template" >&2
    exit 1
  fi
  if [[ ! -f "$starter_src" ]]; then
    echo "Missing starter script: $starter_src" >&2
    exit 1
  fi
  if [[ ! -x "$health_check_src" ]]; then
    echo "Missing health check script or not executable: $health_check_src" >&2
    exit 1
  fi
  if [[ ! -f "$backup_conf_template" ]]; then
    echo "Missing backup config template: $backup_conf_template" >&2
    exit 1
  fi
  if [[ ! -x "$config_backup_src" ]]; then
    echo "Missing config backup script or not executable: $config_backup_src" >&2
    exit 1
  fi
  if [[ ! -x "$upgrade_src" ]]; then
    echo "Missing upgrade script or not executable: $upgrade_src" >&2
    exit 1
  fi
  if [[ ! -f "$passphrase_template" ]]; then
    echo "Missing passphrase file template: $passphrase_template" >&2
    exit 1
  fi
}

build_artifacts() {
  if [[ "$auto_build" != "true" ]]; then
    return 0
  fi
  if ! command -v npm >/dev/null 2>&1; then
    echo "Missing npm command in PATH" >&2
    exit 1
  fi
  if [[ ! -x "$source_dir/web/node_modules/.bin/vite" ]]; then
    echo "Missing frontend builder (vite), installing dependencies..."
    (cd "$source_dir" && npm install --prefix web)
  fi
  if [[ ! -x "$source_dir/web/node_modules/.bin/vite" ]]; then
    echo "vite not found after dependency installation" >&2
    exit 1
  fi
  echo "Building frontend static assets..."
  (cd "$source_dir" && npm run build --prefix web)
  if [[ ! -d "$source_dir/web/dist" ]]; then
    echo "Missing frontend build for embed: $source_dir/web/dist" >&2
    echo "Ensure frontend build outputs web/dist." >&2
    exit 1
  fi
  echo "Building backend binary..."
  mkdir -p "$source_dir/build"
  (cd "$source_dir" && go build -o build/router ./cmd/router)
}

cleanup() {
  if [[ -n "$worktree_dir" && -d "$worktree_dir" ]]; then
    git -C "$root_dir" worktree remove --force "$worktree_dir" >/dev/null 2>&1 || rm -rf "$worktree_dir"
  fi
}
trap cleanup EXIT

target_tag=""
build_ref=""
build_hash_full=""

if [[ -n "$tag_arg" ]]; then
  if ! is_semver_tag "$tag_arg"; then
    usage
    exit 1
  fi
  fetch_remote_refs
  if ! git -C "$root_dir" rev-parse -q --verify "refs/tags/$tag_arg" >/dev/null; then
    echo "Tag not found, skip package: $tag_arg"
    exit 0
  fi
  target_tag="$tag_arg"
  build_ref="$target_tag"
  build_hash_full="$(git -C "$root_dir" rev-list -n 1 "$target_tag")"
  prepare_source_dir "$build_ref"
  build_artifacts
else
  fetch_remote_refs
  remote_main_ref="refs/remotes/$remote_name/main"
  if ! git -C "$root_dir" rev-parse -q --verify "$remote_main_ref" >/dev/null; then
    echo "Missing remote branch: $remote_name/main" >&2
    exit 1
  fi
  max_tag="$(extract_max_tag)"
  main_hash_full="$(git -C "$root_dir" rev-parse "$remote_main_ref")"
  max_tag_hash_full=""
  if [[ -n "$max_tag" ]]; then
    max_tag_hash_full="$(git -C "$root_dir" rev-list -n 1 "$max_tag")"
  fi

  if [[ -n "$max_tag_hash_full" && "$max_tag_hash_full" == "$main_hash_full" ]]; then
    echo "Latest tag $max_tag already matches $remote_name/main HEAD, skip package."
    exit 0
  fi

  if [[ -z "$max_tag" ]]; then
    target_tag="v0.0.1"
  else
    target_tag="$(increment_patch_tag "$max_tag")"
  fi

  if git -C "$root_dir" rev-parse -q --verify "refs/tags/$target_tag" >/dev/null; then
    echo "Tag already exists, refuse to overwrite: $target_tag" >&2
    exit 1
  fi
  build_ref="$main_hash_full"
  build_hash_full="$main_hash_full"
  prepare_source_dir "$build_ref"
  build_artifacts

  git -C "$root_dir" tag "$target_tag" "$main_hash_full"
  if ! git -C "$root_dir" push "$remote_name" "$target_tag"; then
    git -C "$root_dir" tag -d "$target_tag" >/dev/null 2>&1 || true
    echo "Failed to push tag to remote: $target_tag" >&2
    exit 1
  fi
fi

if [[ -z "$build_hash_full" ]]; then
  build_hash_full="$(git -C "$root_dir" rev-parse "$build_ref")"
fi
target_hash="$(git -C "$root_dir" rev-parse --short=7 "$build_hash_full")"
pkg_name="${project_name}-${target_tag}-${target_hash}"
stage_dir="$out_dir/$pkg_name"
archive_path="$out_dir/${pkg_name}.tar.gz"

web_build_dir="$source_dir/web/dist"
if [[ ! -d "$web_build_dir" ]]; then
  echo "Missing frontend build: $source_dir/web/dist" >&2
  echo "Build first: npm run build --prefix web" >&2
  exit 1
fi

verify_artifacts

mkdir -p "$out_dir"
echo "Cleaning output directory: $out_dir"
find "$out_dir" -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +
rm -rf "$stage_dir"
mkdir -p "$stage_dir/build" "$stage_dir/scripts" "$stage_dir/web"

cp "$bin_src" "$stage_dir/build/"
cp "$config_template" "$stage_dir/"
cp "$starter_src" "$stage_dir/scripts/"
cp "$health_check_src" "$stage_dir/scripts/"
cp "$backup_conf_template" "$stage_dir/scripts/"
cp "$config_backup_src" "$stage_dir/scripts/"
  cp "$upgrade_src" "$stage_dir/scripts/"
if [[ -x "$copy_for_upgrade_src" ]]; then
  cp "$copy_for_upgrade_src" "$stage_dir/scripts/"
fi
cp "$passphrase_template" "$stage_dir/scripts/"
cp -R "$web_build_dir" "$stage_dir/web/"

rm -f "$archive_path"
tar -czf "$archive_path" -C "$out_dir" "$pkg_name"

if [[ "${KEEP_STAGE:-0}" != "1" ]]; then
  rm -rf "$stage_dir"
fi

echo "Package created: $archive_path"
