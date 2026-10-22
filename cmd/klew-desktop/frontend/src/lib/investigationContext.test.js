import { describe, expect, it } from 'vitest'
import { inspectRowFromKey, withInspectNamespace } from './investigationContext.js'
import { inspectRowForKey } from './matches.js'
import { inspectScope } from './inspectScope.js'

// Investigation view in klew-lab; the pod is not in any match row.
const view = { state: { namespaceScope: { primary: 'klew-lab' }, snapshot: { namespace: 'klew-lab', pods: [] } } }

function headerFor(row, extra = {}) {
  return inspectScope({ kind: row?.kind, rowNamespace: row?.namespace, ...extra }).label
}

describe('inspector Pod scope on every entry path', () => {
  it('Overview / Graph / signals: short Kind/name keys get the investigation namespace', () => {
    const row = inspectRowFromKey('Pod/payment-api-84bd-j4tsh', view, [])
    expect(row.namespace).toBe('klew-lab')
    expect(headerFor(row)).toBe('klew-lab')
  })

  it('workbench resolver (Graph, cross-links from the inspector)', () => {
    const row = withInspectNamespace(inspectRowForKey('Pod/payment-api-84bd-j4tsh', view, []), view)
    expect(row?.namespace).toBe('klew-lab')
    expect(headerFor(row)).toBe('klew-lab')
  })

  it('Resources list: full Kind/namespace/name keys keep their own namespace', () => {
    const row = inspectRowFromKey('Pod/kube-system/coredns-589f44dc88-jj8d5', view, [])
    expect(row.namespace).toBe('kube-system')
    expect(headerFor(row)).toBe('kube-system')
  })

  it('match rows missing a namespace are filled too', () => {
    const rows = [{ key: 'Pod/p1', kind: 'Pod', name: 'p1', namespace: '' }]
    expect(inspectRowFromKey('Pod/p1', view, rows).namespace).toBe('klew-lab')
  })

  it('cluster-scoped and unknown kinds are not given a namespace', () => {
    expect(inspectRowFromKey('Node/desktop-worker', view, []).namespace).toBe('')
    expect(headerFor(inspectRowFromKey('Node/desktop-worker', view, []))).toBe('Cluster-scoped')
    // A CRD: left to discovery (and the backend detail) instead of guessing.
    expect(inspectRowFromKey('ClusterIssuer/letsencrypt', view, []).namespace).toBe('')
  })

  it('no investigation namespace: header falls back to the detail namespace, never Cluster-scoped', () => {
    const row = inspectRowFromKey('Pod/p1', { state: {} }, [])
    expect(row.namespace).toBe('')
    expect(headerFor(row, { detailNamespace: 'klew-lab' })).toBe('klew-lab')
    expect(headerFor(row)).not.toBe('Cluster-scoped')
  })
})
