import { describe, expect, it } from 'vitest'
import { buildBrowseTabs, tabAppliesToKind } from './inspectTabGuide.js'

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
})
