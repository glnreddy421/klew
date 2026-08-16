#!/usr/bin/env bash
# Notarize Klew.app and staple the ticket.
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

if ! macos_notarization_ready; then
  echo "Notarization credentials not configured; skipping." >&2
  exit 1
fi

echo "Checking notarization credentials..."
if ! xcrun notarytool history \
  --apple-id "$APPLE_ID" \
  --password "$APPLE_APP_PASSWORD" \
  --team-id "$APPLE_TEAM_ID" >/dev/null 2>&1; then
  echo "::error::Notarization credentials rejected by Apple. Regenerate the app-specific password and update APPLE_APP_PASSWORD." >&2
  exit 1
fi

macos_notarize_and_staple "$APP" "app"
