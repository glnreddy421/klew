#!/usr/bin/env bash
# Start `klew mcp` for the Claude plugin. Claude opened from the Dock or a
# launcher often has a PATH without Homebrew, so look in the usual install
# places too. With --check, only report whether klew was found (exit 0 or 1).
set -u

find_klew() {
  if command -v klew >/dev/null 2>&1; then
    command -v klew
    return 0
  fi
  local candidate
  for candidate in \
    /opt/homebrew/bin/klew \
    /usr/local/bin/klew \
    /home/linuxbrew/.linuxbrew/bin/klew \
    "$HOME/.linuxbrew/bin/klew" \
    "$HOME/.local/bin/klew" \
    "$HOME/go/bin/klew"
  do
    if [[ -x "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
  done
  return 1
}

if [[ "${1:-}" == "--check" ]]; then
  find_klew >/dev/null
  exit $?
fi

if ! klew="$(find_klew)"; then
  # stdout carries the MCP protocol, so the reason goes to stderr.
  printf '%s\n' \
    'klew: command not found. Install it, then restart Claude:' \
    '  brew install glnreddy421/klew/klew-cli' \
    'or download a release binary: https://github.com/glnreddy421/klew/releases/latest' >&2
  exit 127
fi

exec "$klew" mcp "$@"
