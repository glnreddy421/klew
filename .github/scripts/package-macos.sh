#!/usr/bin/env bash
# Build Klew.app and package macOS release artifacts (zip + dmg).
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

if ! command -v wails >/dev/null; then
  echo "wails not found; run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0" >&2
  exit 1
fi

# shellcheck source=macos-signing.sh
source "$SCRIPT_DIR/macos-signing.sh"

mkdir -p cmd/klew-desktop/build
if [[ -f "$ROOT/packaging/macos/appicon.png" ]]; then
  cp "$ROOT/packaging/macos/appicon.png" cmd/klew-desktop/build/appicon.png
fi

cd cmd/klew-desktop
npm ci --prefix frontend
npm run build --prefix frontend
wails build -clean -ldflags "$LDFLAGS"

# Wails may wipe build/ during -clean; restore icon for packaging if needed.
if [[ -f "$ROOT/packaging/macos/appicon.png" && ! -f build/appicon.png ]]; then
  cp "$ROOT/packaging/macos/appicon.png" build/appicon.png
fi

APP="build/bin/Klew.app"
if [[ ! -d "$APP" ]]; then
  APP="$(find build/bin -maxdepth 1 -name '*.app' -print -quit)"
fi
if [[ -z "${APP:-}" || ! -d "$APP" ]]; then
  echo "wails build did not produce a .app bundle under build/bin" >&2
  ls -la build/bin >&2 || true
  exit 1
fi

ENTITLEMENTS="$ROOT/packaging/macos/Klew.entitlements"
PKG_STAGING="$(mktemp -d)"
DMG_STAGING=""
cleanup() {
  rm -rf "$PKG_STAGING" "${DMG_STAGING:-}"
}
trap cleanup EXIT

cp -R "$APP" "$PKG_STAGING/Klew.app"

if macos_signing_ready; then
  echo "Signing and notarizing Klew.app with Developer ID..."
  macos_sign_and_notarize_app "$PKG_STAGING/Klew.app" "$ENTITLEMENTS"
else
  echo "Skipping code signing (Apple secrets not configured)."
fi

DIST="$ROOT/dist"
mkdir -p "$DIST"

ARCH="$(uname -m)"
ZIP="$DIST/Klew-${VERSION}-macos-${ARCH}.zip"
DMG="$DIST/Klew-${VERSION}-macos-${ARCH}.dmg"

ditto -c -k --sequesterRsrc --keepParent "$PKG_STAGING/Klew.app" "$ZIP"

DMG_STAGING="$(mktemp -d)"
cp -R "$PKG_STAGING/Klew.app" "$DMG_STAGING/"
ln -s /Applications "$DMG_STAGING/Applications"
hdiutil create -volname "Klew" -srcfolder "$DMG_STAGING" -ov -format UDZO "$DMG"

if macos_signing_ready; then
  echo "Signing and notarizing DMG..."
  macos_sign_and_notarize_dmg "$DMG"
fi

echo "Release artifacts:"
echo "  $ZIP"
echo "  $DMG"
