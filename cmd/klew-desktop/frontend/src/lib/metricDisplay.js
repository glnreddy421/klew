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

/** Like fmtCpu, but whole-core values say "cores" instead of a bare number. */
export function fmtCpuCores(m) {
  const label = fmtCpu(m)
  if (m == null || m < 1000) return label
  return `${label} ${m === 1000 ? 'core' : 'cores'}`
}

function pluralPods(n) {
  return n === 1 ? '1 pod' : `${n} pods`
}

function requestCoverageLabel(covered, total) {
  if (covered == null || total == null || total <= 0) return null
  if (covered >= total) return null
  return `on ${covered} of ${total} pods`
}

function limitCoverageLabel(limitPods, podCount) {
  if (limitPods == null || podCount == null || podCount <= 0 || limitPods <= 0) return null
  if (limitPods >= podCount) return null
  const uncapped = podCount - limitPods
  return `on ${pluralPods(limitPods)} · ${uncapped} uncapped`
}

export function buildOverviewResourceMetric({
  kind,
  available,
  usage,
  request,
  limit,
  usagePods = null,
  requestPods = null,
  limitPods = null,
  podCount = null,
  fmt,
}) {
  // Only put limit on the shared bar when enough pods set one that the
  // totals are roughly comparable. A tiny limit sum next to a full request
  // sum reads as "requests > limits" when it really means "mostly uncapped".
  const limitPartial = Boolean(limit) && limitPods != null && podCount != null && limitPods < podCount
  const limitComparable = Boolean(limit) && !limitPartial && (limit >= (request || 0) || !request)
  const barLimit = limitComparable ? limit : null
  const segments = metricBarSegments({
    usage: available ? usage : null,
    request,
    limit: barLimit,
  })
  // Tone from usage vs request when live; otherwise quiet.
  const tonePct = available && request > 0
    ? utilPct(usage, request)
    : (available ? segments.pct : null)
  const tone = available ? metricToneFromPct(tonePct) : 'muted'
  const usageLabel = available ? fmt(usage) : null
  const requestLabel = request ? fmt(request) : null
  const limitLabel = limit ? fmt(limit) : null
  const primaryLabel = usageLabel || requestLabel || limitLabel
  const primaryKind = usageLabel ? 'usage' : (requestLabel ? 'request' : 'limit')
  return {
    kind,
    available,
    tone,
    ...segments,
    pct: tonePct,
    usageLabel,
    requestLabel,
    limitLabel,
    primaryLabel,
    primaryKind,
    usageCoverage: requestCoverageLabel(usagePods, podCount),
    requestCoverage: requestCoverageLabel(requestPods, podCount),
    limitCoverage: limitCoverageLabel(limitPods, podCount),
    limitPartial,
    allocLabel: requestLabel || limitLabel,
    allocKind: requestLabel ? 'Requested' : 'Limits',
    hasAlloc: (request ?? 0) > 0 || (limit ?? 0) > 0,
    podCount: podCount || 0,
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
      usagePods: summary.cpuUsagePods,
      requestPods: summary.cpuRequestPods,
      limitPods: summary.cpuLimitPods,
      podCount: summary.podCount,
      fmt: fmtCpuCores,
    }),
    memory: buildOverviewResourceMetric({
      kind: 'memory',
      available: summary.available,
      usage: summary.memUsageMi,
      request: summary.memRequestMi,
      limit: summary.memLimitMi,
      usagePods: summary.memUsagePods,
      requestPods: summary.memRequestPods,
      limitPods: summary.memLimitPods,
      podCount: summary.podCount,
      fmt: fmtMem,
    }),
    note: summary.note,
    hasAlloc: summary.hasAlloc,
  }
}
