#!/usr/bin/env bash
# Create/update a GitHub release and upload dist artifacts with retries.
# Verifies assets are publicly downloadable before the workflow continues.
set -euo pipefail

TAG="${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"
REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
TARGET="${GITHUB_SHA:?GITHUB_SHA is required}"
MAX_ATTEMPTS="${RELEASE_UPLOAD_RETRIES:-6}"
WAIT_SEC="${RELEASE_UPLOAD_RETRY_WAIT:-45}"
VERSION="${TAG#v}"

shopt -s nullglob
artifacts=(dist/*)
if ((${#artifacts[@]} == 0)); then
  echo "::error::No files found in dist/"
  exit 1
fi

echo "Publishing ${TAG} to ${REPO} at ${TARGET} (${#artifacts[@]} artifact(s))"

release_exists() {
  gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1
}

release_asset_count() {
  gh release view "$TAG" --repo "$REPO" --json assets -q '.assets | length' 2>/dev/null || echo 0
}

remove_release_if_incomplete() {
  if ! release_exists; then
    return 0
  fi
  local draft count
  draft="$(gh release view "$TAG" --repo "$REPO" --json isDraft -q .isDraft 2>/dev/null || echo false)"
  count="$(release_asset_count)"
  if [[ "$draft" == "true" ]] || (( count < ${#artifacts[@]} )); then
    echo "Removing incomplete release ${TAG} (draft=${draft}, assets=${count})…"
    gh release delete "$TAG" --repo "$REPO" --yes
    sleep 10
  fi
}

create_release() {
  echo "Creating release ${TAG}…"
  gh release create "$TAG" \
    --repo "$REPO" \
    --title "$TAG" \
    --generate-notes \
    --verify-tag \
    --target "$TARGET"
  gh release edit "$TAG" --repo "$REPO" --draft=false --prerelease=false
}

ensure_release() {
  remove_release_if_incomplete
  if release_exists; then
    echo "Release ${TAG} already exists"
    gh release edit "$TAG" --repo "$REPO" --draft=false --prerelease=false || true
    return 0
  fi

  local attempt=1
  while (( attempt <= 5 )); do
    if create_release 2>/dev/null && release_exists; then
      echo "Release ${TAG} is ready"
      return 0
    fi
    echo "Release create did not stick yet (${attempt}/5)…"
    sleep 15
    ((attempt++))
  done

  echo "::error::Failed to create release ${TAG}"
  return 1
}

upload_file() {
  local file="$1"
  local name
  name="$(basename "$file")"
  local attempt=1
  while (( attempt <= MAX_ATTEMPTS )); do
    echo "Uploading ${name} (attempt ${attempt}/${MAX_ATTEMPTS})…"
    if gh release upload "$TAG" "$file" --repo "$REPO" --clobber; then
      echo "Uploaded ${name}"
      return 0
    fi
    echo "::warning::Upload failed for ${name}"
    if (( attempt >= MAX_ATTEMPTS )); then
      echo "::error::Giving up on ${name} after ${MAX_ATTEMPTS} attempts"
      return 1
    fi
    sleep "$WAIT_SEC"
    ((attempt++))
  done
}

verify_public_asset() {
  local name="$1"
  local url="https://github.com/${REPO}/releases/download/${TAG}/${name}"
  local attempt=1
  while (( attempt <= 10 )); do
    local code
    code="$(curl -sL -o /dev/null -w '%{http_code}' "$url")"
    if [[ "$code" == "200" || "$code" == "302" ]]; then
      echo "Verified public download: ${name} (HTTP ${code})"
      return 0
    fi
    echo "Asset not public yet: ${name} (HTTP ${code}), waiting…"
    sleep 15
    ((attempt++))
  done
  echo "::error::Public asset unavailable: ${url}"
  return 1
}

verify_release() {
  local count
  count="$(release_asset_count)"
  if (( count < ${#artifacts[@]} )); then
    echo "::error::Release ${TAG} has ${count} assets, expected ${#artifacts[@]}"
    gh release view "$TAG" --repo "$REPO" --json url,isDraft,assets || true
    return 1
  fi

  verify_public_asset "Klew-${VERSION}-macos-arm64.zip"
  verify_public_asset "Klew-${VERSION}-macos-amd64.zip"
  verify_public_asset "Klew-${VERSION}-macos-arm64.dmg"
  verify_public_asset "Klew-${VERSION}-macos-amd64.dmg"
}

ensure_release

for file in "${artifacts[@]}"; do
  upload_file "$file"
done

gh release edit "$TAG" --repo "$REPO" --draft=false --prerelease=false
sleep 5
verify_release

echo "Release ${TAG} published and verified"
