import { describe, expect, it } from 'vitest'
import { buildOverviewMetricsSummary, fmtCpuCores } from './metricDisplay.js'
import { summarizeBrowsePodMetrics } from './browseMetrics.js'

describe('fmtCpuCores', () => {
  it('names the unit for whole cores and keeps millicores as is', () => {
    expect(fmtCpuCores(41940)).toBe('41.94 cores')
    expect(fmtCpuCores(2000)).toBe('2 cores')
    expect(fmtCpuCores(250)).toBe('250m')
    expect(fmtCpuCores(0)).toBe('—')
  })
})

describe('buildOverviewMetricsSummary', () => {
  const pods = [
    { cpuRequestMilli: 2000, cpuLimitMilli: 4000, memRequestMi: 512, memLimitMi: 1024 },
    { cpuRequestMilli: 1500, memRequestMi: 512 },
  ]

  it('leads with requests for both CPU and memory', () => {
    const { cpu, memory } = buildOverviewMetricsSummary(summarizeBrowsePodMetrics(pods))
    expect(cpu.allocKind).toBe('Requested')
    expect(cpu.allocLabel).toBe('3.50 cores')
    expect(memory.allocKind).toBe('Requested')
    expect(memory.allocLabel).toBe('1Gi')
  })

  it('explains partial limits as uncapped pods, not request > limit', () => {
    const { cpu, memory } = buildOverviewMetricsSummary(summarizeBrowsePodMetrics(pods))
    expect(cpu.limitLabel).toBe('4 cores')
    expect(cpu.limitCoverage).toBe('on 1 pod · 1 uncapped')
    expect(cpu.limitPartial).toBe(true)
    expect(cpu.limitPct).toBeNull()
    expect(memory.limitCoverage).toBe('on 1 pod · 1 uncapped')
  })

  it('omits coverage when every pod sets a limit', () => {
    const { cpu } = buildOverviewMetricsSummary(summarizeBrowsePodMetrics([pods[0]]))
    expect(cpu.limitCoverage).toBeNull()
    expect(cpu.limitPartial).toBe(false)
  })

  it('leads with live usage and measures pressure against requests', () => {
    const livePods = [
      { cpuUsageMilli: 1800, cpuRequestMilli: 2000, cpuLimitMilli: 4000, memUsageMi: 400, memRequestMi: 512 },
      { cpuUsageMilli: 900, cpuRequestMilli: 1000, memUsageMi: 200, memRequestMi: 512 },
    ]
    const { cpu, memory } = buildOverviewMetricsSummary(summarizeBrowsePodMetrics(livePods))
    expect(cpu.available).toBe(true)
    expect(cpu.primaryKind).toBe('usage')
    expect(cpu.usageLabel).toBe('2.70 cores')
    expect(cpu.requestLabel).toBe('3 cores')
    expect(cpu.pct).toBe(90)
    expect(cpu.tone).toBe('warn')
    expect(memory.usageLabel).toBe('600Mi')
    expect(memory.requestCoverage).toBeNull()
  })
})
