#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/package-common.sh"
package_common_init router "$@"
for command_name in go npm; do command -v "$command_name" >/dev/null 2>&1 || package_fail "$command_name is required"; done
(cd "$PACKAGE_ROOT_DIR/web" && npm install --no-audit --no-fund && npm run build)
build_dir="$PACKAGE_STAGE/build"
mkdir -p "$build_dir"
(cd "$PACKAGE_ROOT_DIR" && CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="${GOOS:-linux}" GOARCH="${GOARCH:-amd64}" go build -trimpath -ldflags "-X github.com/yeying-community/router/common.Version=v${PACKAGE_VERSION}" -o "$build_dir/router" ./cmd/router)
package_make_root router
mkdir -p "$PACKAGE_DIR/build"
install -m 0755 "$build_dir/router" "$PACKAGE_DIR/build/router"
for file in config.yaml.template .env.template; do [[ ! -f "$PACKAGE_ROOT_DIR/$file" ]] || cp "$PACKAGE_ROOT_DIR/$file" "$PACKAGE_DIR/"; done
package_copy_scripts "$PACKAGE_DIR/scripts"
package_strip_build_files "$PACKAGE_DIR"
package_finish_artifact service

package_make_root router-web
PACKAGE_HEALTH_PATH=/index.html
cp -R "$PACKAGE_ROOT_DIR/web/dist/." "$PACKAGE_DIR/"
package_finish_artifact static
package_finish
