import { describe, expect, it } from 'vitest'
import {
  bucketsFromFlatTabs,
  createTerminalTab,
  reviveTerminalTabs,
  tabsForCache,
  tabsForContext,
  terminalContextKey,
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

describe('per-context isolation', () => {
  it('filters tabs to one context', () => {
    const tabs = [
      createTerminalTab({ contextName: 'kind-klew-a', title: 'Shell 1' }),
      createTerminalTab({ contextName: 'docker-desktop', title: 'Shell 2' }),
      createTerminalTab({ contextName: 'kind-klew-a', title: 'Shell 3' }),
    ]
    expect(tabsForContext(tabs, 'kind-klew-a').map((t) => t.title)).toEqual(['Shell 1', 'Shell 3'])
    expect(tabsForContext(tabs, 'docker-desktop').map((t) => t.title)).toEqual(['Shell 2'])
    expect(tabsForContext(tabs, 'kind-klew-b')).toEqual([])
  })

  it('splits a flat legacy persist blob into context buckets', () => {
    const tabs = [
      { id: 'a', contextName: 'kind-klew-a', title: 'Shell 1' },
      { id: 'b', contextName: 'docker-desktop', title: 'Shell 2' },
      { id: 'c', contextName: 'kind-klew-a', title: 'Shell 3' },
    ]
    const buckets = bucketsFromFlatTabs(tabs, 'c')
    expect(Object.keys(buckets).sort()).toEqual(['docker-desktop', 'kind-klew-a'])
    expect(buckets['kind-klew-a'].activeId).toBe('c')
    expect(buckets['kind-klew-a'].tabs).toHaveLength(2)
    expect(buckets['docker-desktop'].tabs).toHaveLength(1)
  })

  it('keeps the context name the tab was opened with', () => {
    const tab = createTerminalTab({ contextName: 'kind-klew-a', title: 'Shell 1' })
    expect(tab.contextName).toBe('kind-klew-a')
  })
})
