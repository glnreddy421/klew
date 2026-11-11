import { describe, expect, it } from 'vitest'
import { createElement as h } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { CompareTab } from '../components/inspect/CompareTab.jsx'
import { ChangesTab } from '../components/inspect/WorkloadChanges.jsx'

const now = new Date('2026-10-21T21:00:00Z')
const target = { kind: 'Deployment', namespace: 'klew-lab', name: 'payment-api' }
const wc = {
  kind: 'Deployment', name: 'payment-api', accessState: 'allowed',
  changes: [
    { revision: 14, time: '2026-10-21T20:48:00Z', current: true, summary: 'image v2.3.1 → v2.4.0' },
    { revision: 13, time: '2026-10-20T10:00:00Z', summary: 'env LOG_LEVEL info → debug' },
    { revision: 12, time: '2026-10-19T10:00:00Z', summary: 'Oldest retained revision' },
  ],
}

describe('CompareTab', () => {
  it('offers every revision on both sides, newest against the one before', () => {
    const html = renderToStaticMarkup(h(CompareTab, { state: { data: wc }, target, now }))
    expect(html).toContain('aria-label="From revision"')
    expect(html).toContain('aria-label="To revision"')
    expect(html).toMatch(/<option value="13" selected="">rev 13/)
    expect(html).toMatch(/<option value="14" selected="">rev 14 · current · 12m ago/)
  })

  it('opens the pair the Changes tab asked for', () => {
    const html = renderToStaticMarkup(h(CompareTab, { state: { data: wc }, target, now, requestedPair: { from: 12, to: 14 } }))
    expect(html).toMatch(/<option value="12" selected="">/)
  })

  it('explains when there is nothing to compare', () => {
    const one = { ...wc, changes: [wc.changes[0]] }
    expect(renderToStaticMarkup(h(CompareTab, { state: { data: one }, target, now }))).toContain('only one revision')
    const denied = { accessState: 'denied', accessMessage: 'No permission to read revision history (replicasets list)' }
    expect(renderToStaticMarkup(h(CompareTab, { state: { data: denied }, target, now }))).toContain('No permission')
  })
})

describe('ChangesTab compare link', () => {
  it('still renders rows when given onCompare', () => {
    const html = renderToStaticMarkup(h(ChangesTab, { state: { data: wc }, onCompare: () => {}, now }))
    expect(html).toContain('rev 14')
  })
})
