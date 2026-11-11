import { describe, expect, it } from 'vitest'
import { buildBrowseTabs, foldIntoSummary, tabAppliesToKind } from './inspectTabGuide.js'

describe('inspectTabGuide', () => {
  it('limits containers tab to pods', () => {
    expect(tabAppliesToKind('containers', 'Pod')).toBe(true)
    expect(tabAppliesToKind('containers', 'Deployment')).toBe(false)
  })

  it('builds browse tabs with loading and empty states', () => {
    const tabs = buildBrowseTabs([], { kind: 'Pod' }, { loading: true })
    const summary = tabs.find((t) => t.id === 'summary')
    const containers = tabs.find((t) => t.id === 'containers')
    expect(summary?.state).toBe('loading')
    expect(containers?.state).toBe('loading')
    expect(summary?.description).toMatch(/status/i)
  })

  it('marks populated tabs ready', () => {
    const tabs = buildBrowseTabs(
      [{ id: 'summary', label: 'Summary', sections: [{ id: 's1', title: 'Summary', fields: [{ key: 'Phase', value: 'Running' }] }] }],
      { kind: 'Pod' },
      { loading: false },
    )
    expect(tabs.find((t) => t.id === 'summary')?.state).toBe('ready')
  })

  it('folds spec, runtime and metadata into summary', () => {
    const groups = foldIntoSummary([
      { id: 'summary', label: 'Summary', sections: [{ id: 'summary', title: 'Summary' }] },
      { id: 'metadata', label: 'Metadata', sections: [{ id: 'labels', title: 'Labels' }] },
      { id: 'events', label: 'Events', sections: [{ id: 'events', title: 'Events' }] },
      { id: 'spec', label: 'Spec', sections: [{ id: 'summary', title: 'Strategy' }] },
      { id: 'runtime', label: 'Runtime', sections: [{ id: 'resources', title: 'Resources' }] },
    ])
    expect(groups.map((g) => g.id)).toEqual(['summary', 'events'])
    // Summary, then Spec, Runtime and Metadata, with clashing ids made unique.
    expect(groups[0].sections.map((s) => s.id)).toEqual(['summary', 'spec-summary', 'resources', 'labels'])
  })

  it('builds no spec, runtime or metadata tabs', () => {
    const tabs = buildBrowseTabs(
      [{ id: 'spec', label: 'Spec', sections: [{ id: 'strategy', title: 'Strategy' }] }],
      { kind: 'Pod' },
    )
    const ids = tabs.map((t) => t.id)
    expect(ids).not.toContain('spec')
    expect(ids).not.toContain('runtime')
    expect(ids).not.toContain('metadata')
    expect(tabs.find((t) => t.id === 'summary')?.state).toBe('ready')
  })
})
