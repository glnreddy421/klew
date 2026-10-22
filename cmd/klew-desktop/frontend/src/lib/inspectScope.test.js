import { describe, expect, it } from 'vitest'
import { inspectScope } from './inspectScope.js'
import { readableReason } from './readableReason.js'

describe('inspector scope', () => {
  const tree = { categories: [{ kinds: [{ kind: 'Pod', namespaced: true }, { kind: 'Node', namespaced: false }, { kind: 'Widget', namespaced: false }] }] }

  it('shows the namespace for a Pod opened without one on the row', () => {
    expect(inspectScope({ kind: 'Pod', detailNamespace: 'klew-lab', tree })).toEqual({ namespaced: true, namespace: 'klew-lab', label: 'klew-lab' })
    expect(inspectScope({ kind: 'Pod', fallbackNamespace: 'klew-lab' }).label).toBe('klew-lab')
  })

  it('never calls a namespaced kind cluster-scoped', () => {
    expect(inspectScope({ kind: 'Pod' }).label).not.toBe('Cluster-scoped')
  })

  it('uses discovery for cluster-scoped kinds, including custom ones', () => {
    expect(inspectScope({ kind: 'Node', tree }).label).toBe('Cluster-scoped')
    expect(inspectScope({ kind: 'Widget', tree }).label).toBe('Cluster-scoped')
    expect(inspectScope({ kind: 'Namespace' }).label).toBe('Cluster-scoped')
  })
})

describe('readable reasons', () => {
  it('maps internal identifiers', () => {
    expect(readableReason('metrics_source')).toBe('Metrics signal')
    expect(readableReason('pods_watch_failed')).toBe('Pod watch interrupted')
    expect(readableReason('some_new_thing')).toBe('Some new thing')
    expect(readableReason('BackOff')).toBe('BackOff')
  })
})
