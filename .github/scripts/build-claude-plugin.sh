#!/usr/bin/env bash
# Build Klew's Claude plugin zip: dist/klew-<version>-claude-plugin.zip
#
# Thin plugin: starts `klew` from PATH. The binary is installed separately
# (Homebrew or a release). bin/klew-mcp.sh finds it even when PATH lacks
# Homebrew; a SessionStart hook nudges when klew is missing.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$ROOT"

VERSION="${VERSION#v}"
VERSION="${VERSION:-0.0.0}"
SRC="$ROOT/packaging/claude-plugin"
STAGE="$ROOT/build/claude-plugin"
OUT="$ROOT/dist/klew-${VERSION}-claude-plugin.zip"

rm -rf "$STAGE"
mkdir -p "$STAGE/hooks" "$STAGE/bin" "$STAGE/.claude-plugin" "$ROOT/dist"

cp "$SRC/.claude-plugin/plugin.json" "$STAGE/.claude-plugin/plugin.json"
cp "$SRC/.claude-plugin/icon.png" "$STAGE/.claude-plugin/icon.png"
cp "$SRC/.mcp.json" "$STAGE/.mcp.json"
cp "$SRC/README.md" "$STAGE/README.md"
cp "$SRC/hooks/hooks.json" "$STAGE/hooks/hooks.json"
cp "$SRC/hooks/check-klew.sh" "$STAGE/hooks/check-klew.sh"
cp "$SRC/bin/klew-mcp.sh" "$STAGE/bin/klew-mcp.sh"
chmod +x "$STAGE/hooks/check-klew.sh" "$STAGE/bin/klew-mcp.sh"
cp "$ROOT/LICENSE" "$STAGE/LICENSE"

VERSION="$VERSION" node -e '
  const fs = require("fs");
  const m = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
  m.version = process.env.VERSION;
  fs.writeFileSync(process.argv[1], JSON.stringify(m, null, 2) + "\n");
' "$STAGE/.claude-plugin/plugin.json"

if command -v claude >/dev/null 2>&1; then
  claude plugin validate --strict "$STAGE"
else
  echo "warning: claude CLI not found; skipped plugin validate" >&2
fi

rm -f "$OUT"
(cd "$STAGE" && zip -qr "$OUT" .)
echo "  $OUT"
