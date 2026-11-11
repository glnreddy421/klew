import { describe, expect, it } from 'vitest'
import {
  KLEW_CLI_GO_INSTALL,
  KLEW_CLI_INSTALL,
  installThenInvestigateLine,
  investigateCommand,
} from './klewCli.js'

describe('klewCli', () => {
  it('builds a plain investigate line', () => {
    expect(investigateCommand({})).toBe('klew investigate')
  })

  it('quotes context and namespace', () => {
    expect(investigateCommand({
      klewPath: '/opt/homebrew/bin/klew',
      context: 'docker-desktop',
      namespace: 'klew lab',
    })).toBe("/opt/homebrew/bin/klew investigate -context docker-desktop -n 'klew lab'")
  })

  it('exposes Homebrew and go install lines', () => {
    expect(KLEW_CLI_INSTALL).toContain('brew install')
    expect(KLEW_CLI_INSTALL).toContain('klew-cli')
    expect(KLEW_CLI_GO_INSTALL).toBe('go install github.com/glnreddy421/klew/v2/cmd/klew@latest')
  })
})

describe('installThenInvestigateLine', () => {
  it('runs the install, then stages the investigate command for the next prompt', () => {
    const line = installThenInvestigateLine('brew install klew', "klew investigate -n 'klew lab'")
    expect(line.endsWith('\r')).toBe(true)
    expect(line).toBe(
      `brew install klew && { print -z -- 'klew investigate -n '\\''klew lab'\\''' 2>/dev/null || echo "Next: "'klew investigate -n '\\''klew lab'\\'''; }\r`,
    )
  })
})
