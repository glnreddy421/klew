#!/usr/bin/env bash
# Sign, notarize, and staple Klew.app (and DMG when provided).
set -euo pipefail

macos_signing_ready() {
  [[ -n "${APPLE_CERTIFICATE_BASE64:-}" || -n "${APPLE_SIGNING_IDENTITY:-}" ]] &&
    [[ -n "${APPLE_CERTIFICATE_PASSWORD:-}" ]] &&
    [[ -n "${APPLE_ID:-}" ]] &&
    [[ -n "${APPLE_APP_PASSWORD:-}" ]] &&
    [[ -n "${APPLE_TEAM_ID:-}" ]]
}

macos_resolve_signing_identity() {
  if [[ -n "${APPLE_SIGNING_IDENTITY:-}" ]]; then
    if security find-identity -v -p codesigning 2>/dev/null | grep -Fq "$APPLE_SIGNING_IDENTITY"; then
      echo "$APPLE_SIGNING_IDENTITY"
      return 0
    fi
    echo "Configured APPLE_SIGNING_IDENTITY not found; auto-detecting..." >&2
  fi

  local identity
  identity="$(
    security find-identity -v -p codesigning 2>/dev/null |
      awk -F'"' '/Developer ID Application/ { print $2; exit }'
  )"
  if [[ -z "$identity" ]]; then
    echo "No Developer ID Application identity found in keychain:" >&2
    security find-identity -v -p codesigning >&2 || true
    exit 1
  fi
  echo "$identity"
}

macos_sign_app() {
  local app="$1"
  local entitlements="$2"
  local identity
  identity="$(macos_resolve_signing_identity)"
  echo "Signing with identity: $identity"

  codesign --force --deep --options runtime --timestamp \
    --entitlements "$entitlements" \
    --sign "$identity" \
    "$app"
  codesign --verify --deep --strict --verbose=2 "$app"
}

macos_notarize_and_staple() {
  local artifact="$1"
  local label="$2"
  local submit_path="$artifact"

  if [[ -d "$artifact" ]]; then
    submit_path="${RUNNER_TEMP:-/tmp}/klew-notarize-${label}.zip"
    ditto -c -k --keepParent "$artifact" "$submit_path"
  fi

  echo "Submitting ${label} for notarization..."
  if ! xcrun notarytool submit "$submit_path" \
    --apple-id "$APPLE_ID" \
    --password "$APPLE_APP_PASSWORD" \
    --team-id "$APPLE_TEAM_ID" \
    --wait; then
    echo "Notarization failed for ${label}. Recent submissions:" >&2
    xcrun notarytool history \
      --apple-id "$APPLE_ID" \
      --password "$APPLE_APP_PASSWORD" \
      --team-id "$APPLE_TEAM_ID" 2>&1 | head -20 >&2 || true
    exit 1
  fi

  if [[ -d "$artifact" ]]; then
    xcrun stapler staple "$artifact"
    rm -f "$submit_path"
  else
    xcrun stapler staple "$artifact"
  fi
  echo "Notarized and stapled ${label}."
}

macos_sign_and_notarize_app() {
  local app="$1"
  local entitlements="$2"

  macos_sign_app "$app" "$entitlements"
  macos_notarize_and_staple "$app" "app"
}

macos_sign_and_notarize_dmg() {
  local dmg="$1"
  local identity
  identity="$(macos_resolve_signing_identity)"

  codesign --force --timestamp --sign "$identity" "$dmg"
  macos_notarize_and_staple "$dmg" "dmg"
}
