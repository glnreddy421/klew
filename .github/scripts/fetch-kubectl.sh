#!/usr/bin/env bash
# Download kubectl for macOS into a destination path and verify it against the
# SHA-256 published next to it on dl.k8s.io. Fails without installing anything
# on a missing checksum or a mismatch.
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
TMP="${DEST}.download"
trap 'rm -f "$TMP"' EXIT
curl -fsSL "$URL" -o "$TMP"
WANT="$(curl -fsSL "${URL}.sha256" | awk '{print tolower($1)}')"
if ! [[ "$WANT" =~ ^[0-9a-f]{64}$ ]]; then
  echo "malformed or missing checksum at ${URL}.sha256" >&2
  exit 1
fi
GOT="$(shasum -a 256 "$TMP" | awk '{print $1}')"
if [ "$GOT" != "$WANT" ]; then
  echo "kubectl checksum mismatch: got $GOT, want $WANT" >&2
  exit 1
fi
echo "Verified SHA-256 $GOT"
chmod +x "$TMP"
mv "$TMP" "$DEST"

VERSION_FILE="$(dirname "$DEST")/version.txt"
echo "$VERSION" > "$VERSION_FILE"
if out="$( "$DEST" version --client 2>/dev/null )"; then
  echo "Installed kubectl ${VERSION} -> $DEST"
  echo "$out" | head -1
else
  echo "Installed kubectl ${VERSION} -> $DEST"
fi
