import { describe, expect, it } from 'vitest'
import { terminalNotice } from './terminalBanner'

describe('terminalNotice', () => {
  it('prints nothing for a normal pinned shell', () => {
    expect(terminalNotice({ shell: '/bin/zsh', context: 'kind-dev', namespace: 'apps' })).toBe('')
  })

  it('shows the fallback notice in yellow', () => {
    expect(terminalNotice({ notice: 'No cluster context pinned' })).toBe('\x1b[33mNo cluster context pinned\x1b[0m\r\n')
  })

  it('strips control characters', () => {
    expect(terminalNotice({ notice: 'bad\x1b]0;x\x07' })).toContain('bad]0;x')
  })
})
