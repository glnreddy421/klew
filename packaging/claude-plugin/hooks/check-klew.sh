#!/usr/bin/env bash
# SessionStart: nudge once when klew is missing. Never block the session.
# Uses the same lookup as the MCP launcher, so the two always agree.
set -u

root="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"
if bash "$root/bin/klew-mcp.sh" --check; then
  exit 0
fi

printf '%s\n' \
  'Klew is not installed, so the plugin MCP server cannot start.' \
  'Install the klew command, then restart this session:' \
  '' \
  '  brew install glnreddy421/klew/klew-cli' \
  '' \
  'Or download a release binary: https://github.com/glnreddy421/klew/releases/latest' \
  'Docs: https://github.com/glnreddy421/klew/blob/main/docs/mcp.md'
exit 0
