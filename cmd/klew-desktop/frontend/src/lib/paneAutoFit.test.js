import { describe, expect, it } from 'vitest'
import {
  estimateSingleLineWidth,
  explorerWidthForRow,
  percentile,
  resourceNameColumnWidth,
} from './paneAutoFit.js'

describe('paneAutoFit', () => {
  it('grows with longer strings', () => {
    const short = estimateSingleLineWidth('pod-a', { min: 100, max: 800 })
    const long = estimateSingleLineWidth('kube-controller-manager-desktop-control-plane', { min: 100, max: 800 })
    expect(long).toBeGreaterThan(short)
  })

  it('sizes resource name column from entities', () => {
    const w = resourceNameColumnWidth([
      { name: 'a' },
      { name: 'kube-controller-manager-desktop-control-plane' },
    ])
    expect(w).toBeGreaterThan(300)
  })

  it('takes percentiles without being skewed by outliers', () => {
    const widths = [90, 100, 110, 120, 130, 140, 150, 160, 170, 400]
    expect(percentile(widths, 0.9)).toBe(170)
    expect(percentile(widths, 1)).toBe(400)
    expect(percentile([], 0.9)).toBe(null)
  })

  it('derives the explorer width a row needs from its flexing label', () => {
    // 260px explorer, label box 150px, text 180px → needs 30px more.
    expect(explorerWidthForRow({ explorerWidth: 260, labelWidth: 150, textWidth: 180 })).toBe(290)
  })
})
