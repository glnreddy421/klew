#!/usr/bin/env bash
# Build Klew.app (unsigned). Signing happens in sign-macos.sh.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"

VERSION="${VERSION#v}"
VERSION="${VERSION:-0.0.0}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w \
  -X github.com/glnreddy421/klew/internal/version.Version=${VERSION} \
  -X github.com/glnreddy421/klew/internal/version.Commit=${COMMIT} \
  -X github.com/glnreddy421/klew/internal/version.Date=${DATE}"

export PATH="$(go env GOPATH)/bin:${PATH:-}"

GOARCH="${GOARCH:-arm64}"
case "$GOARCH" in
  arm64 | amd64) ;;
  *)
    echo "unsupported GOARCH: $GOARCH (expected arm64 or amd64)" >&2
    exit 1
    ;;
esac

if ! command -v wails >/dev/null; then
  echo "wails not found; run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0" >&2
  exit 1
fi

mkdir -p cmd/klew-desktop/build
if [[ -f "$ROOT/packaging/macos/appicon.png" ]]; then
  cp "$ROOT/packaging/macos/appicon.png" cmd/klew-desktop/build/appicon.png
fi

cd cmd/klew-desktop
npm ci --prefix frontend
npm run build --prefix frontend
wails build -clean -platform "darwin/${GOARCH}" -ldflags "$LDFLAGS"

APP="build/bin/Klew.app"
if [[ ! -d "$APP" ]]; then
  APP="$(find build/bin -maxdepth 1 -name '*.app' -print -quit)"
fi
if [[ -z "${APP:-}" || ! -d "$APP" ]]; then
  echo "wails build did not produce a .app bundle under build/bin" >&2
  ls -la build/bin >&2 || true
  exit 1
fi

echo "Built ${APP} (${GOARCH})"
