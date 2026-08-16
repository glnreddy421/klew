#!/usr/bin/env bash
# Import Developer ID cert, sign, notarize, and staple Klew.app (and DMG when provided).
set -euo pipefail

macos_signing_ready() {
  [[ -n "${APPLE_CERTIFICATE_BASE64:-}" ]] &&
    [[ -n "${APPLE_CERTIFICATE_PASSWORD:-}" ]] &&
    [[ -n "${APPLE_SIGNING_IDENTITY:-}" ]] &&
    [[ -n "${APPLE_ID:-}" ]] &&
    [[ -n "${APPLE_APP_PASSWORD:-}" ]] &&
    [[ -n "${APPLE_TEAM_ID:-}" ]]
}

macos_setup_signing_keychain() {
  KEYCHAIN_PATH="${RUNNER_TEMP:-/tmp}/klew-signing.keychain-db"
  KEYCHAIN_PASSWORD="${KEYCHAIN_PASSWORD:-$(openssl rand -base64 32)}"

  security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"
  security set-keychain-settings -lut 21600 "$KEYCHAIN_PATH"
  security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"

  CERT_PATH="${RUNNER_TEMP:-/tmp}/klew-cert.p12"
  echo "$APPLE_CERTIFICATE_BASE64" | base64 -D > "$CERT_PATH"
  security import "$CERT_PATH" -P "$APPLE_CERTIFICATE_PASSWORD" -A -t cert -f pkcs12 \
    -k "$KEYCHAIN_PATH" -T /usr/bin/codesign -T /usr/bin/security
  security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN_PATH"

  security list-keychains -d user -s "$KEYCHAIN_PATH" $(security list-keychains -d user | tr -d '"')
  security default-keychain -s "$KEYCHAIN_PATH"
  security find-identity -v -p codesigning "$KEYCHAIN_PATH" || true
}

macos_sign_app() {
  local app="$1"
  local entitlements="$2"

  codesign --force --deep --options runtime --timestamp \
    --entitlements "$entitlements" \
    --sign "$APPLE_SIGNING_IDENTITY" \
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

  macos_setup_signing_keychain
  macos_sign_app "$app" "$entitlements"
  macos_notarize_and_staple "$app" "app"
}

macos_sign_and_notarize_dmg() {
  local dmg="$1"

  codesign --force --timestamp --sign "$APPLE_SIGNING_IDENTITY" "$dmg"
  macos_notarize_and_staple "$dmg" "dmg"
}
