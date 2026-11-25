#!/usr/bin/env bash
# Build Klew's Claude Desktop extension: dist/klew-<version>.mcpb.
#
# Runs after build-cli.sh and packs its signed (and, in a release, notarized)
# macOS binaries as one universal binary, so the bundle works on Apple silicon
# and Intel. Claude Desktop runs on macOS and Windows and the klew command has
# no Windows build, so the bundle declares macOS only.
#
# Then smoke-tests the bundle the way Claude Desktop runs it (smoke.mjs).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"

VERSION="${VERSION#v}"
VERSION="${VERSION:-0.0.0}"
SRC="$ROOT/packaging/mcpb"
CLI="$ROOT/build/cli"
STAGE="$ROOT/build/mcpb"
OUT="$ROOT/dist/klew-${VERSION}.mcpb"

for arch in arm64 amd64; do
  if [[ ! -x "$CLI/darwin-${arch}/klew" ]]; then
    echo "::error::$CLI/darwin-${arch}/klew is missing; run build-cli.sh first." >&2
    exit 1
  fi
done

(cd "$SRC" && npm ci --no-audit --no-fund --silent)
MCPB="$SRC/node_modules/.bin/mcpb"

rm -rf "$STAGE"
mkdir -p "$STAGE/server" "$ROOT/dist"

# lipo keeps each slice's signature, so the universal binary stays signed.
lipo -create -output "$STAGE/server/klew" "$CLI/darwin-arm64/klew" "$CLI/darwin-amd64/klew"
codesign --verify --verbose=2 "$STAGE/server/klew" 2>&1 || echo "warning: klew in the bundle is unsigned" >&2

cp LICENSE "$STAGE/LICENSE"
sips -z 512 512 packaging/macos/appicon.png --out "$STAGE/icon.png" >/dev/null
VERSION="$VERSION" node -e '
  const fs = require("fs");
  const m = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
  m.version = process.env.VERSION;
  fs.writeFileSync(process.argv[2], JSON.stringify(m, null, 2) + "\n");
' "$SRC/manifest.json" "$STAGE/manifest.json"

"$MCPB" validate "$STAGE/manifest.json"
rm -f "$OUT"
"$MCPB" pack "$STAGE" "$OUT"

# Sign the bundle with the Developer ID Application certificate (the release's
# APPLE_CERTIFICATE_BASE64 .p12), so Claude Desktop names a verified publisher.
sign_bundle() {
  if [[ -z "${APPLE_CERTIFICATE_BASE64:-}" || -z "${APPLE_CERTIFICATE_PASSWORD:-}" ]]; then
    if [[ "${REQUIRE_SIGNING:-0}" == "1" ]]; then
      echo "::error::Signing is required but APPLE_CERTIFICATE_BASE64 or APPLE_CERTIFICATE_PASSWORD is not set." >&2
      exit 1
    fi
    echo "warning: no signing certificate; $OUT is unsigned" >&2
    return 0
  fi
  local tmp bin extra converted=0
  tmp="$(mktemp -d)"
  chmod 700 "$tmp"
  # shellcheck disable=SC2064 # expand tmp now
  trap "rm -rf '$tmp'" EXIT
  (umask 077 && printf '%s' "$APPLE_CERTIFICATE_BASE64" | base64 --decode >"$tmp/identity.p12")
  # Newer .p12 exports need OpenSSL 3; older ones need its -legacy provider.
  for bin in "$(brew --prefix openssl@3 2>/dev/null)/bin/openssl" "$(command -v openssl)"; do
    [[ -x "$bin" ]] || continue
    for extra in "" "-legacy"; do
      # shellcheck disable=SC2086 # extra is one optional flag
      if (umask 077 && "$bin" pkcs12 $extra -in "$tmp/identity.p12" -passin env:APPLE_CERTIFICATE_PASSWORD \
        -nodes -out "$tmp/identity.pem") 2>/dev/null; then
        converted=1
        break 2
      fi
    done
  done
  if [[ "$converted" != "1" ]]; then
    echo "::error::Could not read the signing certificate (.p12) with openssl." >&2
    exit 1
  fi
  OPENSSL="$bin" node "$SRC/sign.mjs" "$OUT" "$tmp/identity.pem"
  rm -rf "$tmp"
  trap - EXIT
}
sign_bundle

node "$SRC/smoke.mjs" "$OUT"
echo "  $OUT"
