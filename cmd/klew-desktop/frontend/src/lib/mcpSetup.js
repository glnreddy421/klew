/**
 * Client configuration for Klew's MCP server, as shown in Set Up MCP.
 * Pure functions so the snippets are tested, not hand-checked.
 */

export const MCP_DOCS_URL = 'https://github.com/glnreddy421/klew/blob/main/docs/mcp.md'
export const MCP_BREW_FORMULA = 'glnreddy421/klew/klew-cli'
export const MCP_SERVER_NAME = 'klew'
/** Where Klew.app is installed; snippets use it rather than wherever this copy runs from. */
export const MCP_APP_PATH = '/Applications/Klew.app/Contents/MacOS/Klew'

/** Where each client keeps its MCP servers, for the hint under the snippet. */
export const MCP_CLIENTS = [
  {
    id: 'claude-code',
    label: 'Claude Code',
    kind: 'command',
    hint: 'Run in a terminal, then start a new Claude Code session: one already open does not pick up the change. Adds Klew for every project.',
    existing: {
      check: 'claude mcp get klew',
      text: 'Says klew already exists? This shows which server and scope it uses. To replace it, run claude mcp remove klew -s with that scope, then the command above again.',
    },
  },
  { id: 'claude-desktop', label: 'Claude Desktop', kind: 'json', hint: 'Add to ~/Library/Application Support/Claude/claude_desktop_config.json, then restart Claude Desktop.' },
  { id: 'other', label: 'Cursor & others', kind: 'json', hint: 'Cursor: ~/.cursor/mcp.json. Other clients take the same command and arguments.' },
]

/** Flags for the chosen options, in a stable order. */
export function mcpFlags({ context = '', noLogSamples = false, noHistory = false } = {}) {
  const flags = []
  if (context) flags.push('-context', context)
  if (noLogSamples) flags.push('-no-log-samples')
  if (noHistory) flags.push('-no-history')
  return flags
}

/**
 * The server command: Klew.app's binary or the klew command, with "mcp".
 * No path is specific to this machine. `shell` is the terminal form, where
 * `command -v` stores the full path, for clients started without PATH.
 */
export function mcpServerCommand({ source = 'app' } = {}) {
  if (source === 'app') return { command: MCP_APP_PATH, args: ['mcp'] }
  return { command: 'klew', args: ['mcp'], shell: '"$(command -v klew)"' }
}

/** Quotes s for a POSIX shell when it holds anything beyond safe characters. */
export function shellQuote(s) {
  const v = String(s ?? '')
  if (v !== '' && /^[A-Za-z0-9_@%+=:,./-]+$/.test(v)) return v
  return `'${v.replace(/'/g, `'\\''`)}'`
}

/**
 * The `claude mcp add` line, at user scope so it works in every project.
 * Options for claude go before the name: everything after `--` is passed to
 * the server, which exits on flags it does not know.
 */
export function claudeCodeCommand(server, flags = []) {
  const head = ['claude', 'mcp', 'add', '--scope', 'user', MCP_SERVER_NAME, '--']
  const command = server.shell || shellQuote(server.command)
  return [...head, command, ...[...server.args, ...flags].map(shellQuote)].join(' ')
}

/** The mcpServers JSON block most config-file clients read. */
export function mcpJsonConfig(server, flags = []) {
  const entry = { command: server.command }
  const args = [...server.args, ...flags]
  if (args.length) entry.args = args
  return JSON.stringify({ mcpServers: { [MCP_SERVER_NAME]: entry } }, null, 2)
}

export function mcpSnippet(clientId, server, flags) {
  const client = MCP_CLIENTS.find((c) => c.id === clientId) || MCP_CLIENTS[0]
  return client.kind === 'command' ? claudeCodeCommand(server, flags) : mcpJsonConfig(server, flags)
}

