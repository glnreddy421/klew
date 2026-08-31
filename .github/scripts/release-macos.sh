#!/usr/bin/env bash
# Build, sign, notarize, and package Klew.app for arm64 and amd64 (Intel).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

for GOARCH in arm64 amd64; do
  export GOARCH
  echo "=== macOS ${GOARCH} ==="
  "$SCRIPT_DIR/build-macos.sh"
  "$SCRIPT_DIR/sign-app.sh"
  "$SCRIPT_DIR/notarize-app.sh"
  "$SCRIPT_DIR/package-dist.sh"
done

echo "All macOS release artifacts:"
ls -la "${SCRIPT_DIR}/../../dist"/Klew-*-macos-* 2>/dev/null || true
