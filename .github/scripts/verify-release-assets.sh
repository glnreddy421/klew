#!/usr/bin/env bash
# Verify a release's assets are published and public WITHOUT downloading them.
#
# Downloading an asset (even `curl -o /dev/null`) increments its GitHub
# download_count, so earlier checks inflated every release's numbers. This
# asks the API instead. An unauthenticated request only sees published
# (non-draft) releases — the public view — so it proves the files are public.
# If the shared runner IP is rate-limited, fall back to an authenticated check
# of the same facts (release not a draft, asset uploaded, size > 0).
#
# Usage: verify-release-assets.sh <tag> <repo> <asset-name>...
set -euo pipefail

TAG="${1:?tag is required}"
REPO="${2:?repo is required}"
shift 2
(( $# > 0 )) || { echo "::error::no asset names given" >&2; exit 2; }

ATTEMPTS="${VERIFY_ATTEMPTS:-10}"
WAIT_SEC="${VERIFY_WAIT_SEC:-15}"
API="https://api.github.com/repos/${REPO}/releases/tags/${TAG}"

# Prints the release JSON from the public API, or nothing (rate limited / not public yet).
public_release_json() {
  local body code
  body="$(mktemp)"
  code="$(curl -s -o "$body" -w '%{http_code}' -H 'Accept: application/vnd.github+json' "$API" || true)"
  if [[ "$code" == "200" ]]; then
    cat "$body"
  elif [[ "$code" == "403" || "$code" == "429" ]]; then
    echo "RATE_LIMITED"
  fi
  rm -f "$body"
}

authenticated_release_json() {
  gh api "repos/${REPO}/releases/tags/${TAG}" 2>/dev/null || true
}

# Exit 0 when every named asset is uploaded with size > 0 in the given JSON.
assets_ok() {
  # The JSON arrives on stdin, so the program goes in -c (a heredoc would take stdin).
  python3 -c '
import json, sys
names = sys.argv[1:]
rel = json.load(sys.stdin)
if rel.get("draft"):
    print("release is still a draft"); sys.exit(1)
assets = {a["name"]: a for a in rel.get("assets", [])}
missing = [n for n in names if n not in assets or assets[n].get("state") != "uploaded" or not assets[n].get("size")]
for n in names:
    if n not in missing:
        print("OK %s (%d bytes, %s)" % (n, assets[n]["size"], assets[n]["state"]))
if missing:
    print("not ready: " + ", ".join(missing)); sys.exit(1)
' "$@"
}

attempt=1
while (( attempt <= ATTEMPTS )); do
  json="$(public_release_json)"
  source="public API"
  if [[ "$json" == "RATE_LIMITED" ]] && command -v gh >/dev/null 2>&1; then
    json="$(authenticated_release_json)"
    source="authenticated API (public API rate-limited)"
  fi
  if [[ -n "$json" && "$json" != "RATE_LIMITED" ]] && printf '%s' "$json" | assets_ok "$@"; then
    echo "Verified ${#} asset(s) for ${TAG} via ${source} — no downloads counted."
    exit 0
  fi
  echo "Release ${TAG} assets not public yet (attempt ${attempt}/${ATTEMPTS}), waiting…"
  sleep "$WAIT_SEC"
  ((attempt++))
done
echo "::error::Release ${TAG} assets are not publicly available: $*"
exit 1
