#!/usr/bin/env bash
# Bump glnreddy421/homebrew-klew after macOS release zips are built.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TAG="${GITHUB_REF_NAME:?GITHUB_REF_NAME is required (e.g. v0.1.4)}"
VERSION="${TAG#v}"
TAP_REPO="${HOMEBREW_TAP_REPO:-glnreddy421/homebrew-klew}"
TAP_BRANCH="${HOMEBREW_TAP_BRANCH:-main}"

if [[ -z "${HOMEBREW_TAP_TOKEN:-}" ]]; then
  echo "HOMEBREW_TAP_TOKEN is not set — add a GitHub PAT with write access to ${TAP_REPO}." >&2
  exit 1
fi

zip_sha() {
  local arch="$1"
  local zip="$ROOT/dist/Klew-${VERSION}-macos-${arch}.zip"
  if [[ ! -f "$zip" ]]; then
    echo "Release zip not found: $zip" >&2
    exit 1
  fi
  shasum -a 256 "$zip" | awk '{print $1}'
}

ARM64_URL="https://github.com/glnreddy421/klew/releases/download/${TAG}/Klew-${VERSION}-macos-arm64.zip"
AMD64_URL="https://github.com/glnreddy421/klew/releases/download/${TAG}/Klew-${VERSION}-macos-amd64.zip"
ARM64_SHA256="$(zip_sha arm64)"
AMD64_SHA256="$(zip_sha amd64)"

WORK="$(mktemp -d)"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

git clone --depth 1 --branch "$TAP_BRANCH" \
  "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" \
  "$WORK/tap"

mkdir -p "$WORK/tap/Casks"
# Retire the legacy formula — GUI apps belong in Casks (installs to /Applications, clean upgrades).
rm -f "$WORK/tap/Formula/klew.rb"
rmdir "$WORK/tap/Formula" 2>/dev/null || true

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
if git diff --staged --quiet; then
  echo "Homebrew cask already up to date."
  exit 0
fi
git commit -m "klew ${VERSION} (cask)"
git push origin "$TAP_BRANCH"

echo "Updated ${TAP_REPO} cask to klew ${VERSION} (arm64 + Intel)."
