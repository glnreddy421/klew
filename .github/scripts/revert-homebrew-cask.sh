#!/usr/bin/env bash
# Revert the Homebrew cask to a known-good release (default: 1.3.4).
set -euo pipefail

VERSION="${1:-1.3.4}"
TAG="v${VERSION}"
TAP_REPO="${HOMEBREW_TAP_REPO:-glnreddy421/homebrew-klew}"
TAP_BRANCH="${HOMEBREW_TAP_BRANCH:-main}"

if [[ -z "${HOMEBREW_TAP_TOKEN:-}" ]]; then
  echo "HOMEBREW_TAP_TOKEN is not set." >&2
  exit 1
fi

case "$VERSION" in
  1.3.4)
    ARM64_SHA256="951bf39cdb5fb983706db70134827df138f8298eb54f09ff1f0b026a4c00dc47"
    AMD64_SHA256="2c38060cdd6697999977a54a4f16e3308298a1405ae74825443e491831e3ccaf"
    ;;
  *)
    echo "No baked-in checksums for version ${VERSION}; run update-homebrew-tap.sh after a good release instead." >&2
    exit 1
    ;;
esac

ARM64_URL="https://github.com/glnreddy421/klew/releases/download/${TAG}/Klew-${VERSION}-macos-arm64.zip"
AMD64_URL="https://github.com/glnreddy421/klew/releases/download/${TAG}/Klew-${VERSION}-macos-amd64.zip"

WORK="$(mktemp -d)"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

git clone --depth 1 --branch "$TAP_BRANCH" \
  "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" \
  "$WORK/tap"

mkdir -p "$WORK/tap/Casks"
cat > "$WORK/tap/Casks/klew.rb" <<EOF
cask "klew" do
  version "${VERSION}"

  on_arm do
    sha256 "${ARM64_SHA256}"
    url "${ARM64_URL}"
  end
  on_intel do
    sha256 "${AMD64_SHA256}"
    url "${AMD64_URL}"
  end

  name "Klew"
  desc "Live Kubernetes incident investigation (desktop app)"
  homepage "https://github.com/glnreddy421/klew"

  app "Klew.app"

  livecheck do
    url "https://github.com/glnreddy421/klew/releases/latest"
    strategy :github_latest
  end
end
EOF

cd "$WORK/tap"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git add -A
git commit -m "Revert klew cask to ${VERSION}"
git push origin "$TAP_BRANCH"

echo "Reverted ${TAP_REPO} cask to klew ${VERSION}"
