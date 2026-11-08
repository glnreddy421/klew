#!/usr/bin/env bash
# Bump glnreddy421/homebrew-klew after the release archives are built: the
# klew cask (desktop app) and the klew-cli formula (the klew command).
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

tar_sha() {
  local target="$1"
  local tarball="$ROOT/dist/klew-cli-${VERSION}-${target}.tar.gz"
  if [[ ! -f "$tarball" ]]; then
    echo "Release archive not found: $tarball" >&2
    exit 1
  fi
  shasum -a 256 "$tarball" | awk '{print $1}'
}

ARM64_URL="https://github.com/glnreddy421/klew/releases/download/${TAG}/Klew-${VERSION}-macos-arm64.zip"
AMD64_URL="https://github.com/glnreddy421/klew/releases/download/${TAG}/Klew-${VERSION}-macos-amd64.zip"
ARM64_SHA256="$(zip_sha arm64)"
AMD64_SHA256="$(zip_sha amd64)"

CLI_BASE="https://github.com/glnreddy421/klew/releases/download/${TAG}/klew-cli-${VERSION}"
CLI_DARWIN_ARM64_SHA256="$(tar_sha darwin-arm64)"
CLI_DARWIN_AMD64_SHA256="$(tar_sha darwin-amd64)"
CLI_LINUX_ARM64_SHA256="$(tar_sha linux-arm64)"
CLI_LINUX_AMD64_SHA256="$(tar_sha linux-amd64)"

WORK="$(mktemp -d)"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

git clone --depth 1 --branch "$TAP_BRANCH" \
  "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" \
  "$WORK/tap"

mkdir -p "$WORK/tap/Casks" "$WORK/tap/Formula"
# Retire the legacy klew formula — the GUI app belongs in Casks (installs to
# /Applications, clean upgrades). Formula/ now holds only klew-cli.
rm -f "$WORK/tap/Formula/klew.rb"

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

cat > "$WORK/tap/Formula/klew-cli.rb" <<EOF
class KlewCli < Formula
  desc "Kubernetes incident investigation, with a read-only MCP server"
  homepage "https://github.com/glnreddy421/klew"
  version "${VERSION}"
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "${CLI_BASE}-darwin-arm64.tar.gz"
      sha256 "${CLI_DARWIN_ARM64_SHA256}"
    end
    on_intel do
      url "${CLI_BASE}-darwin-amd64.tar.gz"
      sha256 "${CLI_DARWIN_AMD64_SHA256}"
    end
  end

  on_linux do
    on_arm do
      url "${CLI_BASE}-linux-arm64.tar.gz"
      sha256 "${CLI_LINUX_ARM64_SHA256}"
    end
    on_intel do
      url "${CLI_BASE}-linux-amd64.tar.gz"
      sha256 "${CLI_LINUX_AMD64_SHA256}"
    end
  end

  def install
    bin.install "klew"
  end

  def caveats
    <<~EOS
      Investigate a namespace:
        klew investigate -n NAMESPACE
      Add it to Claude Code:
        claude mcp add --scope user klew -- "\$(command -v klew)" mcp
      More: https://github.com/glnreddy421/klew/blob/main/docs/cli.md
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/klew version")
  end
end
EOF

cd "$WORK/tap"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git add -A
if git diff --staged --quiet; then
  echo "Homebrew tap already up to date."
  exit 0
fi
git commit -m "klew ${VERSION} (cask, klew-cli formula)"
git push origin "$TAP_BRANCH"

echo "Updated ${TAP_REPO}: klew cask and klew-cli formula ${VERSION}."
