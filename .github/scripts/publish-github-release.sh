#!/usr/bin/env bash
# Create/update a GitHub release and upload dist artifacts with retries.
# Handles transient GitHub API 5xx responses during large asset uploads.
set -euo pipefail

TAG="${GITHUB_REF_NAME:?GITHUB_REF_NAME is required}"
REPO="${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
MAX_ATTEMPTS="${RELEASE_UPLOAD_RETRIES:-6}"
WAIT_SEC="${RELEASE_UPLOAD_RETRY_WAIT:-45}"

shopt -s nullglob
artifacts=(dist/*)
if ((${#artifacts[@]} == 0)); then
  echo "::error::No files found in dist/"
  exit 1
fi

echo "Publishing ${TAG} to ${REPO} (${#artifacts[@]} artifact(s))"

release_exists() {
  gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1
}

ensure_release() {
  local attempt=1
  while (( attempt <= 10 )); do
    if release_exists; then
      echo "Release ${TAG} is ready"
      return 0
    fi
    if (( attempt == 1 )); then
      echo "Creating release ${TAG}…"
      gh release create "$TAG" \
        --repo "$REPO" \
        --title "$TAG" \
        --generate-notes \
        || true
    fi
    echo "Waiting for release ${TAG} to become visible (${attempt}/10)…"
    sleep 15
    ((attempt++))
  done
  echo "::error::Timed out waiting for release ${TAG}"
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

ensure_release

for file in "${artifacts[@]}"; do
  upload_file "$file"
done

echo "Release ${TAG} assets uploaded successfully"
