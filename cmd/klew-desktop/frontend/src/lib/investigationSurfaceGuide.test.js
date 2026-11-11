import { describe, expect, it } from 'vitest'
import { getInvestigationSurface, INVESTIGATION_SURFACES } from './investigationSurfaceGuide.js'

describe('investigationSurfaceGuide', () => {
  it('defines all investigation surfaces', () => {
    expect(Object.keys(INVESTIGATION_SURFACES).sort()).toEqual(['evidence', 'failures', 'overview', 'patterns'])
  })

  it('includes actionable steps for patterns', () => {
    const surface = getInvestigationSurface('patterns')
    expect(surface.steps.join(' ')).toMatch(/Investigate/i)
    expect(surface.provides.length).toBeGreaterThan(0)
  })

  it('gives each surface a short journey and outcomes for the empty state', () => {
    for (const surface of Object.values(INVESTIGATION_SURFACES)) {
      expect(surface.journey).toHaveLength(3)
      expect(surface.outcomes.length).toBeGreaterThan(0)
      expect(surface.cue).toMatch(/Investigate/i)
      expect(surface.headline.length).toBeGreaterThan(8)
    }
  })
})
