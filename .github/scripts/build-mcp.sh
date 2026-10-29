#!/usr/bin/env bash
# Build klew-mcp release archives for macOS and Linux (arm64 + amd64).
#
# Binaries land in build/mcp/<os>-<arch>/klew-mcp (scanned for the SBOM) and
# archives in dist/klew-mcp-<version>-<os>-<arch>.tar.gz (published).
# macOS binaries are signed with the Developer ID identity and notarized when
# signing is available; REQUIRE_SIGNING=1 (set by the release workflow) makes
# missing signing an error instead of a warning.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"

# shellcheck source=macos-signing.sh
source "$SCRIPT_DIR/macos-signing.sh"

VERSION="${VERSION#v}"
VERSION="${VERSION:-0.0.0}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo none)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-s -w \
  -X github.com/glnreddy421/klew/internal/version.Version=${VERSION} \
  -X github.com/glnreddy421/klew/internal/version.Commit=${COMMIT} \
  -X github.com/glnreddy421/klew/internal/version.Date=${DATE}"

TARGETS=(darwin/arm64 darwin/amd64 linux/arm64 linux/amd64)
OUT="$ROOT/build/mcp"
DIST="$ROOT/dist"
rm -rf "$OUT"
mkdir -p "$OUT" "$DIST"

for target in "${TARGETS[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  dir="$OUT/${goos}-${goarch}"
  mkdir -p "$dir"
  echo "Building klew-mcp ${VERSION} (${goos}/${goarch})..."
  # Pure Go (SQLite via modernc.org/sqlite), so no cgo toolchain is needed.
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$dir/klew-mcp" ./cmd/klew-mcp
done

if macos_signing_ready; then
  identity="$(macos_resolve_signing_identity)"
  for goarch in arm64 amd64; do
    codesign --force --options runtime --timestamp --sign "$identity" "$OUT/darwin-${goarch}/klew-mcp"
    codesign --verify --verbose=2 "$OUT/darwin-${goarch}/klew-mcp"
  done

  # A bare Mach-O binary cannot carry a stapled ticket; Gatekeeper checks the
  # notarization online on first run.
  macos_normalize_apple_credentials
  submit="${RUNNER_TEMP:-/tmp}/klew-mcp-notarize.zip"
  rm -f "$submit"
  (cd "$OUT" && zip -q -r "$submit" darwin-arm64/klew-mcp darwin-amd64/klew-mcp)
  echo "Submitting klew-mcp for notarization..."
  xcrun notarytool submit "$submit" \
    --apple-id "$APPLE_ID" \
    --password "$APPLE_APP_PASSWORD" \
    --team-id "$APPLE_TEAM_ID" \
    --wait
  rm -f "$submit"
elif [[ "${REQUIRE_SIGNING:-0}" == "1" ]]; then
  echo "::error::Signing is required but no Developer ID identity or Apple credentials are available." >&2
  exit 1
else
  echo "Signing prerequisites not met; macOS klew-mcp binaries are unsigned." >&2
fi

for target in "${TARGETS[@]}"; do
  goos="${target%/*}"
  goarch="${target#*/}"
  dir="$OUT/${goos}-${goarch}"
  cp LICENSE "$dir/LICENSE"
  archive="$DIST/klew-mcp-${VERSION}-${goos}-${goarch}.tar.gz"
  tar -C "$dir" -czf "$archive" klew-mcp LICENSE
  echo "  $archive"
done
