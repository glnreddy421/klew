#!/usr/bin/env bash
# Bump glnreddy421/homebrew-klew after a macOS release zip is built.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TAG="${GITHUB_REF_NAME:?GITHUB_REF_NAME is required (e.g. v0.1.4)}"
VERSION="${TAG#v}"
ARCH="${ARCH:-arm64}"

ZIP_NAME="Klew-${VERSION}-macos-${ARCH}.zip"
ZIP_PATH="${ZIP_PATH:-$ROOT/dist/$ZIP_NAME}"
TAP_REPO="${HOMEBREW_TAP_REPO:-glnreddy421/homebrew-klew}"
TAP_BRANCH="${HOMEBREW_TAP_BRANCH:-main}"

if [[ -z "${HOMEBREW_TAP_TOKEN:-}" ]]; then
  echo "HOMEBREW_TAP_TOKEN is not set — add a GitHub PAT with write access to ${TAP_REPO}." >&2
  exit 1
fi

if [[ ! -f "$ZIP_PATH" ]]; then
  echo "Release zip not found: $ZIP_PATH" >&2
  exit 1
fi

SHA256="$(shasum -a 256 "$ZIP_PATH" | awk '{print $1}')"
URL="https://github.com/glnreddy421/klew/releases/download/${TAG}/${ZIP_NAME}"

WORK="$(mktemp -d)"
cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT

git clone --depth 1 --branch "$TAP_BRANCH" \
  "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" \
  "$WORK/tap"

cat > "$WORK/tap/Formula/klew.rb" <<EOF
class Klew < Formula
  desc "Klew — live Kubernetes incident investigation (desktop app)"
  homepage "https://github.com/glnreddy421/klew"
  license "Apache-2.0"
  version "${VERSION}"

  depends_on :macos

  on_macos do
    on_arm do
      url "${URL}"
      sha256 "${SHA256}"
    end
  end

  def install
    app = if (buildpath/"Contents/MacOS").directory?
      buildpath
    elsif (buildpath/"Klew.app").directory?
      buildpath/"Klew.app"
    else
      buildpath.glob("*.app").first
    end
    odie "Klew.app not found under #{buildpath}" unless app&.directory?

    cp_r app, prefix/"Klew.app"
  end

  def caveats
    <<~EOS
      Klew.app is installed at:
        #{prefix}/Klew.app

      Launch from Finder or run:
        open #{prefix}/Klew.app
    EOS
  end

  test do
    assert_path_exists prefix/"Klew.app"
    assert_path_exists prefix/"Klew.app/Contents/MacOS/Klew"
  end
end
EOF

cd "$WORK/tap"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git add Formula/klew.rb
if git diff --staged --quiet; then
  echo "Homebrew formula already up to date."
  exit 0
fi
git commit -m "klew ${VERSION}"
git push origin "$TAP_BRANCH"

echo "Updated ${TAP_REPO} to klew ${VERSION} (${SHA256})."
