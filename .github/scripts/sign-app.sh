#!/usr/bin/env bash
# Sign Klew.app in place under cmd/klew-desktop/build/bin/.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"

# shellcheck source=macos-signing.sh
source "$SCRIPT_DIR/macos-signing.sh"

APP="cmd/klew-desktop/build/bin/Klew.app"
if [[ ! -d "$APP" ]]; then
  APP="$(find cmd/klew-desktop/build/bin -maxdepth 1 -name '*.app' -print -quit)"
fi
if [[ -z "${APP:-}" || ! -d "$APP" ]]; then
  echo "Klew.app not found — run build-macos.sh first" >&2
  exit 1
fi

if ! macos_signing_ready; then
  echo "Signing prerequisites not met; skipping." >&2
  exit 1
fi

ENTITLEMENTS="$ROOT/packaging/macos/Klew.entitlements"
macos_sign_app "$APP" "$ENTITLEMENTS"
