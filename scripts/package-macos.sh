#!/usr/bin/env bash
# Build Klew.app and package macOS release artifacts (zip + dmg).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
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

if ! command -v wails >/dev/null; then
  echo "wails not found; run: make install-desktop-tools" >&2
  exit 1
fi

cd cmd/klew-desktop
npm ci --prefix frontend
npm run build --prefix frontend
wails build -clean -ldflags "$LDFLAGS"

APP="build/bin/Klew.app"
if [[ ! -d "$APP" ]]; then
  APP="$(find build/bin -maxdepth 1 -name '*.app' -print -quit)"
fi
if [[ -z "${APP:-}" || ! -d "$APP" ]]; then
  echo "wails build did not produce a .app bundle under build/bin" >&2
  ls -la build/bin >&2 || true
  exit 1
fi

DIST="$ROOT/dist"
mkdir -p "$DIST"

ARCH="$(uname -m)"
ZIP="$DIST/Klew-${VERSION}-macos-${ARCH}.zip"
DMG="$DIST/Klew-${VERSION}-macos-${ARCH}.dmg"

PKG_STAGING="$(mktemp -d)"
cleanup() { rm -rf "$PKG_STAGING" "$DMG_STAGING"; }
trap cleanup EXIT

cp -R "$APP" "$PKG_STAGING/Klew.app"
ditto -c -k --sequesterRsrc --keepParent "$PKG_STAGING/Klew.app" "$ZIP"

DMG_STAGING="$(mktemp -d)"
cp -R "$PKG_STAGING/Klew.app" "$DMG_STAGING/"
ln -s /Applications "$DMG_STAGING/Applications"
hdiutil create -volname "Klew" -srcfolder "$DMG_STAGING" -ov -format UDZO "$DMG"

echo "Release artifacts:"
echo "  $ZIP"
echo "  $DMG"
