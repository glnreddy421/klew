#!/usr/bin/env bash
# Fail fast when Apple notarization credentials are missing or rejected.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=macos-signing.sh
source "$SCRIPT_DIR/macos-signing.sh"

if ! macos_notarization_ready; then
  echo "::error::Missing notarization secrets. Set APPLE_ID, APPLE_TEAM_ID, and APPLE_APP_PASSWORD (app-specific password) in GitHub repo secrets." >&2
  exit 1
fi

macos_normalize_apple_credentials

echo "Checking notarization credentials for Apple ID ${APPLE_ID} (team ${APPLE_TEAM_ID})..."
if xcrun notarytool history \
  --apple-id "$APPLE_ID" \
  --password "$APPLE_APP_PASSWORD" \
  --team-id "$APPLE_TEAM_ID" >/dev/null 2>&1; then
  echo "Notarization credentials accepted by Apple."
  exit 0
fi

echo "::error::Notarization credentials rejected by Apple." >&2
cat >&2 <<'EOF'
Fix:
1. Sign in at https://appleid.apple.com → Sign-In and Security → App-Specific Passwords
2. Generate a NEW password (label e.g. "Klew GitHub Actions")
3. In GitHub: klew repo → Settings → Secrets and variables → Actions
4. Update secret APPLE_APP_PASSWORD with the generated xxxx-xxxx-xxxx-xxxx value
   (NOT your normal Apple ID password; no spaces; paste once)
5. Confirm APPLE_ID is your Apple ID email and APPLE_TEAM_ID is your 10-char Team ID
6. Re-run the Release workflow or re-push the tag
EOF
exit 1
