import { describe, expect, it } from 'vitest'
import { resolveDisplayEntities } from './catalogDisplay.js'

describe('resolveDisplayEntities', () => {
  const lazy = [{
    key: 'DaemonSet/kube-system/kube-proxy',
    kind: 'DaemonSet',
    name: 'kube-proxy',
    namespace: 'kube-system',
    desiredReplicas: 2,
    readyReplicas: 2,
    misscheduled: 0,
  }]

  const investigation = [{
    key: 'DaemonSet/kube-proxy',
    kind: 'DaemonSet',
    name: 'kube-proxy',
    namespace: 'kube-system',
    signal: '2/2 ready',
  }]

  it('merges catalog status onto investigation rows in matches lens', () => {
    const rows = resolveDisplayEntities({
      catalogAll: false,
      investigationEntities: investigation,
      lazyEntities: lazy,
    })
    expect(rows[0].desiredReplicas).toBe(2)
    expect(rows[0].misscheduled).toBe(0)
  })

  it('uses lazy entities when investigation list is empty', () => {
    expect(resolveDisplayEntities({
      catalogAll: false,
      investigationEntities: [],
      lazyEntities: lazy,
    })).toEqual(lazy)
  })
})
