import { describe, expect, it } from 'vitest'
import {
  addOrUpdateLogTab,
  buildDockTabItems,
  findMatchingLogTab,
  logTabKey,
  parseDockTabKey,
  removeTabFromDock,
  syncTerminalTabsInOrder,
  tabsRightOf,
  termTabKey,
} from './dockTabs'

describe('dockTabs pure state helpers', () => {
  it('parses terminal and log tab keys', () => {
    expect(parseDockTabKey(termTabKey('term-tab-1'))).toEqual({ kind: 'terminal', id: 'term-tab-1' })
    expect(parseDockTabKey(logTabKey('log-2'))).toEqual({ kind: 'log', id: 'log-2' })
    expect(parseDockTabKey('other')).toBeNull()
  })

  it('interleaves terminal and log tabs and preserves order on terminal changes', () => {
    let state = { tabOrder: [], logTabs: [], activeKey: null }
    state.tabOrder = syncTerminalTabsInOrder(state.tabOrder, [{ id: 't1', title: 'Shell 1' }])
    state = addOrUpdateLogTab(state, {
      streamId: 'log-1',
      spec: { targets: [{ namespace: 'shop', pods: ['api-7f9c-x2'] }] },
    })
    state.tabOrder = syncTerminalTabsInOrder(state.tabOrder, [
      { id: 't1', title: 'Shell 1' },
      { id: 't2', title: 'Shell 2' },
    ])
    state = addOrUpdateLogTab(state, {
      streamId: 'log-2',
      spec: { targets: [{ namespace: 'shop', kind: 'Deployment', name: 'checkout' }] },
    })

    expect(state.tabOrder).toEqual(['term:t1', 'log:log-1', 'term:t2', 'log:log-2'])
    expect(state.activeKey).toBe('log:log-2')

    const items = buildDockTabItems(
      state.tabOrder,
      [
        { id: 't1', title: 'Shell 1', ready: true },
        { id: 't2', title: 'Shell 2', ready: false },
      ],
      state.logTabs,
      {
        'log-1': { status: { active: true, paused: false } },
        'log-2': { status: { active: true, paused: true } },
      },
    )
    expect(items.map((i) => [i.kind, i.title, i.state || ''])).toEqual([
      ['terminal', 'Shell 1', ''],
      ['log', 'api-7f9c-x2', 'live'],
      ['terminal', 'Shell 2', ''],
      ['log', 'Deployment/checkout', 'paused'],
    ])
  })

  it('deduplicates log tabs by selection spec regardless of pod order', () => {
    let state = { tabOrder: [], logTabs: [], activeKey: null }
    state = addOrUpdateLogTab(state, {
      streamId: 'log-1',
      spec: { targets: [{ namespace: 'shop', pods: ['pod-b', 'pod-a'] }] },
    })
    const dup = findMatchingLogTab(state.logTabs, {
      targets: [{ namespace: 'shop', pods: ['pod-a', 'pod-b'] }],
    })
    expect(dup?.streamId).toBe('log-1')
    expect(dup?.title).toBe('2 sources')

    const diff = findMatchingLogTab(state.logTabs, {
      targets: [{ namespace: 'shop', pods: ['pod-a'] }],
    })
    expect(diff).toBeNull()
  })

  it('picks adjacent tab when closing active tab', () => {
    let state = {
      tabOrder: ['term:t1', 'log:log-1', 'log:log-2'],
      logTabs: [
        { streamId: 'log-1', selectionKey: 'k1', title: 'a' },
        { streamId: 'log-2', selectionKey: 'k2', title: 'b' },
      ],
      activeKey: 'log:log-1',
    }
    state = removeTabFromDock(state, 'log:log-1')
    expect(state.tabOrder).toEqual(['term:t1', 'log:log-2'])
    expect(state.activeKey).toBe('log:log-2')
    expect(state.logTabs.map((t) => t.streamId)).toEqual(['log-2'])

    state = removeTabFromDock(state, 'log:log-2')
    expect(state.tabOrder).toEqual(['term:t1'])
    expect(state.activeKey).toBe('term:t1')
  })

  it('lists tabs to the right of a key', () => {
    const tabs = [{ key: 'term:t1' }, { key: 'log:log-1' }, { key: 'term:t2' }]
    expect(tabsRightOf(tabs, 'term:t1')).toEqual(['log:log-1', 'term:t2'])
    expect(tabsRightOf(tabs, 'term:t2')).toEqual([])
    expect(tabsRightOf(tabs, 'missing')).toEqual([])
  })

  it('folds over several keys for close others / close all', () => {
    const start = {
      tabOrder: ['term:t1', 'term:t2', 'log:log-1'],
      logTabs: [{ streamId: 'log-1', spec: {}, sources: [] }],
      activeKey: 'term:t1',
    }

    const others = ['term:t1', 'log:log-1']
      .reduce((acc, key) => removeTabFromDock(acc, key), start)
    expect(others.tabOrder).toEqual(['term:t2'])
    expect(others.activeKey).toBe('term:t2')
    expect(others.logTabs).toEqual([])

    const all = start.tabOrder.reduce((acc, key) => removeTabFromDock(acc, key), start)
    expect(all.tabOrder).toEqual([])
    expect(all.activeKey).toBeNull()
  })
})
