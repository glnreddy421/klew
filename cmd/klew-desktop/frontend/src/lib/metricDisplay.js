import { fmtCpu, fmtMem, utilPct } from './investigationViews.js'

export function metricBarSegments({ usage = null, request = 0, limit = 0 } = {}) {
  const u = usage == null ? null : Number(usage) || 0
  const req = Number(request) || 0
  const lim = Number(limit) || 0
  const denom = Math.max(lim, req, u ?? 0, 1)
  return {
    denom,
    usagePct: u != null && u > 0 ? Math.min(100, (u / denom) * 100) : null,
    requestPct: req > 0 ? Math.min(100, (req / denom) * 100) : null,
    limitPct: lim > 0 ? Math.min(100, (lim / denom) * 100) : null,
    pct: u != null ? utilPct(u, denom) : null,
  }
}

export function metricToneFromPct(pct) {
  if (pct == null) return 'muted'
  if (pct >= 95) return 'crit'
  if (pct >= 80) return 'warn'
  return 'ok'
}

export function buildPodCpuMetric(row) {
  const usage = row?.cpuUsageMilli
  const request = row?.cpuRequestMilli
  const limit = row?.cpuLimitMilli
  const segments = metricBarSegments({ usage, request, limit })
  const hasLive = usage != null && usage > 0
  const hasAlloc = (request ?? 0) > 0 || (limit ?? 0) > 0
  return {
    ...segments,
    hasLive,
    hasAlloc,
    usage,
    request,
    limit,
    label: hasLive ? fmtCpu(usage) : (hasAlloc ? fmtCpu(limit || request) : '—'),
    title: formatMetricTitle('CPU', { usage, request, limit, fmt: fmtCpu, live: hasLive }),
  }
}

export function buildPodMemMetric(row) {
  const usage = row?.memUsageMi
  const request = row?.memRequestMi
  const limit = row?.memLimitMi
  const segments = metricBarSegments({ usage, request, limit })
  const hasLive = usage != null && usage > 0
  const hasAlloc = (request ?? 0) > 0 || (limit ?? 0) > 0
  return {
    ...segments,
    hasLive,
    hasAlloc,
    usage,
    request,
    limit,
    label: hasLive ? fmtMem(usage) : (hasAlloc ? fmtMem(limit || request) : '—'),
    title: formatMetricTitle('Memory', { usage, request, limit, fmt: fmtMem, live: hasLive }),
  }
}

function formatMetricTitle(kind, { usage, request, limit, fmt, live }) {
  const parts = []
  if (live && usage != null) parts.push(`usage ${fmt(usage)}`)
  if (request) parts.push(`request ${fmt(request)}`)
  if (limit) parts.push(`limit ${fmt(limit)}`)
  if (!parts.length) return `${kind}: no data`
  return `${kind}: ${parts.join(' · ')}`
}

export function buildOverviewResourceMetric({
  kind,
  available,
  usage,
  request,
  limit,
  fmt,
}) {
  const segments = metricBarSegments({ usage: available ? usage : null, request, limit })
  const tone = available ? metricToneFromPct(segments.pct) : 'muted'
  const denomLabel = fmt(segments.denom)
  const usageLabel = available ? fmt(usage) : null
  return {
    kind,
    available,
    tone,
    ...segments,
    usageLabel,
    denomLabel,
    requestLabel: request ? fmt(request) : null,
    limitLabel: limit ? fmt(limit) : null,
    hasAlloc: (request ?? 0) > 0 || (limit ?? 0) > 0,
  }
}

export function buildOverviewMetricsSummary(summary) {
  return {
    cpu: buildOverviewResourceMetric({
      kind: 'cpu',
      available: summary.available,
      usage: summary.cpuUsageMillicores,
      request: summary.cpuRequestMillicores,
      limit: summary.cpuLimitMillicores,
      fmt: fmtCpu,
    }),
    memory: buildOverviewResourceMetric({
      kind: 'memory',
      available: summary.available,
      usage: summary.memUsageMi,
      request: summary.memRequestMi,
      limit: summary.memLimitMi,
      fmt: fmtMem,
    }),
    note: summary.note,
    hasAlloc: summary.hasAlloc,
  }
}
