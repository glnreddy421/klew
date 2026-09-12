import { fmtCpu, fmtMem, utilPct } from './investigationViews.js'

export function summarizeBrowsePodMetrics(podEntities = []) {
  let cpuUsageMillicores = 0
  let memUsageMi = 0
  let cpuRequestMillicores = 0
  let memRequestMi = 0
  let cpuLimitMillicores = 0
  let memLimitMi = 0
  let available = false

  for (const entity of podEntities) {
    if (entity.cpuUsageMilli != null) {
      cpuUsageMillicores += Number(entity.cpuUsageMilli) || 0
      available = true
    }
    if (entity.memUsageMi != null) {
      memUsageMi += Number(entity.memUsageMi) || 0
      available = true
    }
    if (entity.cpuRequestMilli != null) {
      cpuRequestMillicores += Number(entity.cpuRequestMilli) || 0
    }
    if (entity.cpuLimitMilli != null) {
      cpuLimitMillicores += Number(entity.cpuLimitMilli) || 0
    }
    if (entity.memRequestMi != null) {
      memRequestMi += Number(entity.memRequestMi) || 0
    }
    if (entity.memLimitMi != null) {
      memLimitMi += Number(entity.memLimitMi) || 0
    }
  }

  const hasAlloc = cpuRequestMillicores > 0 || cpuLimitMillicores > 0 || memRequestMi > 0 || memLimitMi > 0
  const cpuDenom = Math.max(cpuLimitMillicores, cpuRequestMillicores, cpuUsageMillicores, 1)
  const memDenom = Math.max(memLimitMi, memRequestMi, memUsageMi, 1)

  let note = 'Install metrics-server for live CPU/memory usage'
  if (available) {
    note = 'metrics-server'
  } else if (hasAlloc) {
    note = 'Showing pod requests/limits — install metrics-server for live usage'
  }

  return {
    available,
    hasAlloc,
    note,
    cpuUsageMillicores,
    memUsageMi,
    cpuRequestMillicores,
    memRequestMi,
    cpuLimitMillicores,
    memLimitMi,
    cpuPct: available ? utilPct(cpuUsageMillicores, cpuDenom) : null,
    memPct: available ? utilPct(memUsageMi, memDenom) : null,
    cpuLabel: fmtCpu(cpuUsageMillicores),
    memLabel: fmtMem(memUsageMi),
    cpuDenomLabel: fmtCpu(cpuDenom),
    memDenomLabel: fmtMem(memDenom),
  }
}
