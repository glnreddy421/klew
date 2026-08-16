#!/usr/bin/env bash
# Sign, notarize, and package Klew.app into dist/ zip + dmg.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"

# shellcheck source=macos-signing.sh
source "$SCRIPT_DIR/macos-signing.sh"

VERSION="${VERSION#v}"
VERSION="${VERSION:-0.0.0}"
APP="cmd/klew-desktop/build/bin/Klew.app"
if [[ ! -d "$APP" ]]; then
  APP="$(find cmd/klew-desktop/build/bin -maxdepth 1 -name '*.app' -print -quit)"
fi
if [[ -z "${APP:-}" || ! -d "$APP" ]]; then
  echo "Klew.app not found — run build-macos.sh first" >&2
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
