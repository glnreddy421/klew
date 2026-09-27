import { describe, expect, it, beforeEach } from 'vitest'
import {
  createTerminalTab,
  reviveTerminalTabs,
  terminalContextKey,
  tabsForCache,
} from './useTerminalTabs.js'

describe('terminalContextKey', () => {
  it('combines kubeconfig path and context', () => {
    expect(terminalContextKey({
      kubeconfigPath: '/a/kube',
      selectedContext: 'prod',
    })).toBe('/a/kube\0prod')
  })
})

describe('reviveTerminalTabs', () => {
  it('clears session state', () => {
    const tabs = reviveTerminalTabs([
      { id: 't1', sessionId: 'pty-1', ready: true, title: 'Shell 1' },
    ])
    expect(tabs[0].sessionId).toBeNull()
    expect(tabs[0].ready).toBe(false)
    expect(tabs[0].restartToken).toBeTruthy()
  })
})

describe('tabsForCache', () => {
  it('strips runtime fields', () => {
    const tab = createTerminalTab({ contextName: 'dev', namespace: 'default' })
    tab.sessionId = 'x'
    tab.ready = true
    const cached = tabsForCache([tab])
    expect(cached[0]).toEqual({
      id: tab.id,
      title: tab.title,
      contextName: 'dev',
      namespace: 'default',
      initialInput: '',
      initialInputSent: false,
    })
  })
})
