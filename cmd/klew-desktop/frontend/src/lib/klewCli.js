/**
 * Helpers for launching or installing the standalone `klew` command from Home.
 */

import { MCP_BREW_FORMULA, shellQuote } from './mcpSetup.js'

export const KLEW_CLI_BREW_FORMULA = MCP_BREW_FORMULA
export const KLEW_CLI_INSTALL = `brew install ${MCP_BREW_FORMULA}`
export const KLEW_CLI_GO_INSTALL = 'go install github.com/glnreddy421/klew/v2/cmd/klew@latest'
/** Upgrade when Homebrew already manages klew, otherwise install it. */
export const KLEW_CLI_BREW_UPDATE = `brew upgrade ${MCP_BREW_FORMULA} 2>/dev/null || brew install ${MCP_BREW_FORMULA}`
export const KLEW_CLI_DOCS_URL = 'https://github.com/glnreddy421/klew/blob/main/docs/cli.md'

/**
 * Shell line for `klew investigate` (no trailing newline — user confirms with Enter).
 * Prefers an absolute path when LookPath found one outside a thin Dock PATH.
 */
export function investigateCommand({
  klewPath = 'klew',
  context = '',
  namespace = '',
} = {}) {
  const bin = shellQuote(String(klewPath || 'klew').trim() || 'klew')
  const parts = [bin, 'investigate']
  const ctx = String(context || '').trim()
  const ns = String(namespace || '').trim()
  if (ctx) parts.push('-context', shellQuote(ctx))
  if (ns) parts.push('-n', shellQuote(ns))
  return parts.join(' ')
}

/**
 * One shell line that installs klew and, when the install succeeds, leaves the
 * investigate command typed at the next prompt (zsh `print -z`) for the user
 * to review and run. Shells without `print -z` echo it instead.
 * Ends with a carriage return: clicking an install chip runs the install.
 */
export function installThenInvestigateLine(installCmd, investigateCmd) {
  const next = shellQuote(investigateCmd)
  return `${installCmd} && { print -z -- ${next} 2>/dev/null || echo "Next: "${next}; }\r`
}
