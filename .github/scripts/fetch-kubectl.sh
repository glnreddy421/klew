#!/usr/bin/env bash
# Download kubectl for macOS into a destination path.
# Usage: fetch-kubectl.sh <arm64|amd64> <output-path>
set -euo pipefail

ARCH="${1:?GOARCH required (arm64 or amd64)}"
DEST="${2:?destination path required}"
VERSION="${KUBECTL_VERSION:-v1.31.4}"

case "$ARCH" in
  arm64 | amd64) ;;
  *)
    echo "unsupported arch: $ARCH" >&2
    exit 1
    ;;
esac

mkdir -p "$(dirname "$DEST")"
URL="https://dl.k8s.io/release/${VERSION}/bin/darwin/${ARCH}/kubectl"
echo "Fetching kubectl ${VERSION} (${ARCH})..."
curl -fsSL "$URL" -o "$DEST"
chmod +x "$DEST"

VERSION_FILE="$(dirname "$DEST")/version.txt"
echo "$VERSION" > "$VERSION_FILE"
if out="$( "$DEST" version --client 2>/dev/null )"; then
  echo "Installed kubectl ${VERSION} -> $DEST"
  echo "$out" | head -1
else
  echo "Installed kubectl ${VERSION} -> $DEST"
fi
