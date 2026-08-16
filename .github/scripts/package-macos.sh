#!/usr/bin/env bash
# Build, sign, notarize, and package macOS release artifacts (zip + dmg).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
"$SCRIPT_DIR/build-macos.sh"
"$SCRIPT_DIR/sign-macos.sh"
