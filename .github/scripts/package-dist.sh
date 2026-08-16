#!/usr/bin/env bash
# Package a signed or unsigned Klew.app into dist/ zip + dmg.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"

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

DIST="$ROOT/dist"
mkdir -p "$DIST"
ARCH="$(uname -m)"
ZIP="$DIST/Klew-${VERSION}-macos-${ARCH}.zip"
DMG="$DIST/Klew-${VERSION}-macos-${ARCH}.dmg"

ditto -c -k --sequesterRsrc --keepParent "$APP" "$ZIP"

DMG_STAGING="$(mktemp -d)"
cleanup() { rm -rf "$DMG_STAGING"; }
trap cleanup EXIT

cp -R "$APP" "$DMG_STAGING/"
ln -s /Applications "$DMG_STAGING/Applications"
hdiutil create -volname "Klew" -srcfolder "$DMG_STAGING" -ov -format UDZO "$DMG"

echo "Release artifacts:"
echo "  $ZIP"
echo "  $DMG"
