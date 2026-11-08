import { describe, expect, it } from 'vitest'
import {
  MCP_APP_PATH,
  claudeCodeCommand,
  mcpFlags,
  mcpJsonConfig,
  mcpServerCommand,
  mcpSnippet,
  shellQuote,
} from './mcpSetup.js'

const APP = '/Applications/Klew.app/Contents/MacOS/Klew'

describe('mcpSetup', () => {
  it('builds flags in a stable order', () => {
    expect(mcpFlags()).toEqual([])
    expect(mcpFlags({ context: 'kind-lab', noLogSamples: true, noHistory: true }))
      .toEqual(['-context', 'kind-lab', '-no-log-samples', '-no-history'])
  })

  it('uses paths that are the same on every machine', () => {
    expect(MCP_APP_PATH).toBe(APP)
    expect(mcpServerCommand({ source: 'app' })).toEqual({ command: APP, args: ['mcp'] })
    expect(mcpServerCommand({ source: 'standalone' }))
      .toEqual({ command: 'klew', args: ['mcp'], shell: '"$(command -v klew)"' })
  })

  it('quotes only what the shell needs quoted', () => {
    expect(shellQuote('kind-lab')).toBe('kind-lab')
    expect(shellQuote('arn:aws:eks:us-east-1:123:cluster/prod')).toBe('arn:aws:eks:us-east-1:123:cluster/prod')
    expect(shellQuote('/Applications/My Klew.app/x')).toBe(`'/Applications/My Klew.app/x'`)
    expect(shellQuote(`it's`)).toBe(`'it'\\''s'`)
    expect(shellQuote('')).toBe(`''`)
  })

  it('writes the claude mcp add line', () => {
    expect(claudeCodeCommand(mcpServerCommand({ source: 'app' }), mcpFlags({ noLogSamples: true })))
      .toBe(`claude mcp add --scope user klew -- ${APP} mcp -no-log-samples`)
    expect(claudeCodeCommand(mcpServerCommand({ source: 'standalone' }), mcpFlags({ context: 'my ctx' })))
      .toBe(`claude mcp add --scope user klew -- "$(command -v klew)" mcp -context 'my ctx'`)
  })

  it('writes the mcpServers JSON, without empty args or shell syntax', () => {
    expect(JSON.parse(mcpJsonConfig(mcpServerCommand({ source: 'standalone' }))))
      .toEqual({ mcpServers: { klew: { command: 'klew', args: ['mcp'] } } })
    expect(JSON.parse(mcpJsonConfig(mcpServerCommand({ source: 'app' }), ['-no-history'])))
      .toEqual({ mcpServers: { klew: { command: APP, args: ['mcp', '-no-history'] } } })
  })

  it('picks the snippet by client', () => {
    const server = mcpServerCommand({ source: 'standalone' })
    expect(mcpSnippet('claude-code', server, [])).toBe('claude mcp add --scope user klew -- "$(command -v klew)" mcp')
    expect(mcpSnippet('claude-desktop', server, [])).toContain('"mcpServers"')
  })
})
