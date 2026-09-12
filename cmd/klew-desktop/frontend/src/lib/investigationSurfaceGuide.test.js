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
})
