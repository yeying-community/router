#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

SWAG_BIN="$(command -v swag || true)"
if [ -z "$SWAG_BIN" ]; then
  SWAG_BIN="$(go env GOPATH)/bin/swag"
fi

"$SWAG_BIN" init --generalInfo docs/swagger/swagger.go --output docs/swagger --parseDependency --parseInternal

if [ -f docs/swagger/swagger.json ]; then
  mv docs/swagger/swagger.json docs/swagger/openapi.json
fi
