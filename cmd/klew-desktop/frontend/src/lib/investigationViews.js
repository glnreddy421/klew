/**
 * Shared derivations for Timeline / Graph / Failures / Resources / Evidence.
 * Ports TUI ranking and health labels — never invents telemetry.
 */

export function getState(view) {
  return view?.state || {}
}

export function getSnapshot(view) {
  return getState(view).snapshot || {}
}

export function formatClock(ts) {
  if (!ts) return '—'
  try {
    return new Date(ts).toLocaleTimeString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false,
    })
  } catch {
    return '—'
  }
}

export function formatClockDate(ts) {
  if (!ts) return '—'
  try {
    const d = new Date(ts)
    return `${d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })} ${formatClock(ts)}`
  } catch {
    return '—'
  }
}

export function normalizeHealth(h) {
  const s = String(h || '').toLowerCase()
  if (s === 'critical' || s === 'crit') return 'critical'
  if (s === 'warning' || s === 'warn' || s === 'degraded') return 'warning'
  if (s === 'healthy' || s === 'ok' || s === 'ready') return 'healthy'
  return 'unknown'
}

export function healthRank(h) {
  switch (normalizeHealth(h)) {
    case 'critical': return 3
    case 'warning': return 2
    case 'unknown': return 1
    default: return 0
  }
}

export function podHealthLabel(p) {
  for (const c of p?.containers || []) {
    const lr = String(c.lastReason || '').toLowerCase()
    if (c.lastReason === 'OOMKilled' || lr.includes('crash')) return 'critical'
  }
  if (p?.ready && p?.phase === 'Running') return 'healthy'
  if ((p?.restartCount || 0) > 3) return 'critical'
  if (!p?.ready) return 'warning'
  return 'unknown'
}

export function containerHealthLabel(c) {
  const lr = String(c?.lastReason || '').toLowerCase()
  if (c?.lastReason === 'OOMKilled' || lr.includes('crash')) return 'critical'
  if (!c?.ready) return 'warning'
  return 'healthy'
}

export function worstContainer(p) {
  const list = p?.containers || []
  if (!list.length) return null
  return list.reduce((best, c) => {
    const br = healthRank(containerHealthLabel(best))
    const cr = healthRank(containerHealthLabel(c))
    if (cr > br || (cr === br && (c.restartCount || 0) > (best.restartCount || 0))) return c
    return best
  })
}

export function podTriageScore(p) {
  let score = healthRank(podHealthLabel(p)) * 100
  score += (p.restartCount || 0) * 5
  if (!p.ready) score += 20
  const c = worstContainer(p)
  if (c?.lastReason === 'OOMKilled') score += 30
  return score
}

export function rankPodsForTriage(pods) {
  return [...(pods || [])].sort((a, b) => {
    const d = podTriageScore(b) - podTriageScore(a)
    if (d) return d
    if ((b.restartCount || 0) !== (a.restartCount || 0)) {
      return (b.restartCount || 0) - (a.restartCount || 0)
    }
    return String(a.name).localeCompare(String(b.name))
  })
}

export function blastRadiusCounts(snapshot) {
  const pods = snapshot?.pods || []
  let critical = 0
  let warning = 0
  let healthy = 0
  let podsFailing = 0
  for (const p of pods) {
    const h = podHealthLabel(p)
    if (h === 'critical') { critical++; podsFailing++ }
    else if (h === 'warning') { warning++; podsFailing++ }
    else if (h === 'healthy') healthy++
    else warning++
  }
  for (const w of snapshot?.workloads || []) {
    const ready = w.ready ?? w.readyReplicas ?? 0
    const desired = w.replicas ?? w.desired ?? 0
    if (desired > 0 && ready < desired) warning++
    else if (desired > 0) healthy++
  }
  for (const svc of snapshot?.services || []) {
    const er = svc.endpointsReady ?? svc.readyEndpoints
    const et = svc.endpointsTotal ?? svc.totalEndpoints
    if (et != null && er != null && et > 0 && er < et) warning++
    else healthy++
  }
  return {
    critical,
    warning,
    healthy,
    pods: pods.length,
    podsFailing,
  }
}

export function severityRankTL(sev) {
  switch (String(sev || '').toLowerCase()) {
    case 'critical':
    case 'error':
    case 'fatal':
      return 3
    case 'warning':
    case 'warn':
      return 2
    case 'info':
      return 1
    default:
      return 0
  }
}

function isDeployEvent(ev) {
  const t = `${ev?.type || ''} ${ev?.reason || ''} ${ev?.message || ''}`.toLowerCase()
  return /deploy|rollout|scaled|created|image.*updated|replicaset.*created/.test(t)
}

function isRollbackEvent(ev) {
  const t = `${ev?.type || ''} ${ev?.reason || ''} ${ev?.message || ''}`.toLowerCase()
  return /rollback|roll.?back|undo/.test(t)
}

function isRecoveryEvent(ev) {
  const t = `${ev?.type || ''} ${ev?.reason || ''} ${ev?.message || ''}`.toLowerCase()
  return /recover|became.?ready|started|healthy|succeeded/.test(t)
    && severityRankTL(ev?.severity) < 3
}

function isHypoChange(ev) {
  return String(ev?.type || '').toLowerCase() === 'hypothesis'
    || /hypothesis/i.test(ev?.reason || '')
}

/**
 * Fold identical consecutive timeline events; annotate phase bookmarks.
 */
export function buildTimelineRuns(events) {
  const evs = [...(events || [])].sort((a, b) => {
    const ta = new Date(a.timestamp || 0).getTime()
    const tb = new Date(b.timestamp || 0).getTime()
    return ta - tb
  })

  const runs = []
  for (const ev of evs) {
    const last = runs[runs.length - 1]
    const key = `${ev.type}|${ev.reason}|${ev.sourceKind}|${ev.sourceName}|${ev.message}`
    if (last && last.key === key) {
      last.count += 1
      last.last = ev.timestamp
      last.ev = ev
      continue
    }
    runs.push({
      key,
      ev,
      count: 1,
      first: ev.timestamp,
      last: ev.timestamp,
    })
  }

  let deployIdx = -1
  let failureIdx = -1
  let peakIdx = -1
  let rollbackIdx = -1
  let recoveryIdx = -1
  let lastFailIdx = -1
  let peakScore = -1

  runs.forEach((r, i) => {
    if (deployIdx < 0 && isDeployEvent(r.ev)) deployIdx = i
    if (rollbackIdx < 0 && isRollbackEvent(r.ev)) rollbackIdx = i
    const rk = severityRankTL(r.ev.severity)
    if (rk >= 3) {
      if (failureIdx < 0) failureIdx = i
      lastFailIdx = i
      const sc = rk * 1000 + r.count
      if (sc > peakScore) {
        peakScore = sc
        peakIdx = i
      }
    }
  })

  if (peakIdx >= 0) {
    for (let i = peakIdx + 1; i < runs.length; i++) {
      if (isRecoveryEvent(runs[i].ev)) {
        recoveryIdx = i
        break
      }
    }
  }

  const annotated = runs.map((r, i) => {
    let annotation = ''
    if (i === deployIdx) annotation = 'trigger'
    else if (i === peakIdx) annotation = 'peak'
    else if (i === rollbackIdx) annotation = 'rollback'
    else if (i === recoveryIdx) annotation = 'recovery'
    const inChain = lastFailIdx >= 0 && i >= Math.max(0, deployIdx) && i <= lastFailIdx
    const phase = phaseAt(i, deployIdx, failureIdx)
    const hypo = isHypoChange(r.ev)
    return { ...r, annotation, inChain, phase, hypo }
  })

  return {
    runs: annotated,
    bookmarks: { deployIdx, failureIdx, peakIdx, rollbackIdx, recoveryIdx },
  }
}

function phaseAt(i, deployIdx, failureIdx) {
  if (failureIdx >= 0 && i === failureIdx) return 'Impact'
  if (deployIdx >= 0 && i === deployIdx) return 'Change'
  if (failureIdx >= 0 && i > failureIdx) return 'Aftermath'
  if (deployIdx >= 0 && i > deployIdx && (failureIdx < 0 || i < failureIdx)) return 'Propagation'
  return ''
}

export function fmtCpu(m) {
  if (m == null || m === 0) return '—'
  if (m >= 1000) return `${(m / 1000).toFixed(m % 1000 === 0 ? 0 : 2)}`
  return `${m}m`
}

export function fmtMem(mi) {
  if (mi == null || mi === 0) return '—'
  if (mi >= 1024) return `${(mi / 1024).toFixed(mi % 1024 === 0 ? 0 : 1)}Gi`
  return `${mi}Mi`
}

export function utilPct(usage, denom) {
  if (!denom || denom <= 0 || usage == null) return null
  return Math.min(999, Math.round((usage / denom) * 100))
}

export function confidenceLabel(n) {
  if (n == null || n <= 0) return '—'
  if (n >= 0.85) return 'High'
  if (n >= 0.6) return 'Medium'
  if (n >= 0.35) return 'Low'
  return 'Weak'
}

export function rankedVerdictSignals(verdict) {
  const v = verdict || {}
  return [
    ...(v.strongSignals || []).map((s) => ({ ...s, strength: s.strength || 'strong' })),
    ...(v.mediumSignals || []).map((s) => ({ ...s, strength: s.strength || 'medium' })),
    ...(v.weakSignals || []).map((s) => ({ ...s, strength: s.strength || 'weak' })),
  ]
}

export function groupEvidence(events) {
  const groups = { event: [], change: [], metric: [], other: [] }
  for (const e of events || []) {
    const t = String(e.sourceType || '').toLowerCase()
    if (t === 'log' || t === 'system') continue
    if (t === 'k8s_event' || t === 'event') groups.event.push(e)
    else if (t === 'object_change' || t === 'change') groups.change.push(e)
    else if (t === 'metric' || t === 'metrics') groups.metric.push(e)
    else groups.other.push(e)
  }
  return groups
}

export function investigationWindowLabel(state) {
  const ms = state?.window
  if (!ms || ms <= 0) return '15 minutes'
  const m = Math.round(ms / 60000)
  if (m >= 1) return `${m} minute${m === 1 ? '' : 's'}`
  return `${Math.round(ms / 1000)} seconds`
}

export function associatedResources(snapshot) {
  const snap = snapshot || {}
  const rows = []
  for (const w of snap.workloads || []) {
    rows.push({ kind: w.kind || 'Workload', name: w.name, detail: `${w.ready ?? w.readyReplicas ?? '—'}/${w.replicas ?? w.desired ?? '—'} ready` })
  }
  for (const s of snap.services || []) {
    const er = s.endpointsReady ?? s.readyEndpoints ?? '—'
    const et = s.endpointsTotal ?? s.totalEndpoints ?? '—'
    rows.push({ kind: 'Service', name: s.name, detail: `endpoints ${er}/${et}` })
  }
  for (const i of snap.ingresses || []) {
    rows.push({ kind: 'Ingress', name: i.name, detail: i.hosts?.join(', ') || i.class || '—' })
  }
  for (const h of snap.hpas || []) {
    rows.push({ kind: 'HPA', name: h.name, detail: `${h.currentReplicas ?? '—'} → ${h.desiredReplicas ?? '—'} (max ${h.maxReplicas ?? '—'})` })
  }
  for (const cm of (snap.configMaps || []).slice(0, 8)) {
    rows.push({ kind: 'ConfigMap', name: cm.name || cm, detail: 'config' })
  }
  for (const sec of (snap.secrets || []).slice(0, 8)) {
    rows.push({ kind: 'Secret', name: sec.name || sec, detail: 'secret' })
  }
  return rows
}

export function nodePressureFlags(node) {
  const flags = []
  if (!node) return flags
  if (!node.ready) flags.push('NotReady')
  if (node.memoryPressure) flags.push('MemoryPressure')
  if (node.diskPressure) flags.push('DiskPressure')
  if (node.pidPressure) flags.push('PIDPressure')
  if (node.unschedulable) flags.push('Unschedulable')
  return flags
}

export function resourceFindings(view) {
  const snap = getSnapshot(view)
  const m = snap.metrics || {}
  const pods = snap.pods || []
  const findings = []

  const memDenom = Math.max(m.memLimitMi || 0, m.memRequestMi || 0)
  const cpuDenom = Math.max(m.cpuLimitMillicores || 0, m.cpuRequestMillicores || 0)
  const memPct = utilPct(m.memUsageMi, memDenom)
  const cpuPct = utilPct(m.cpuUsageMillicores, cpuDenom)

  if (m.available && memPct != null && memPct >= 85) {
    findings.push({ level: 'warn', text: `Memory usage at ${memPct}% of limit — OOM risk` })
  }
  if (m.available && cpuPct != null && cpuPct >= 90) {
    findings.push({ level: 'warn', text: `CPU usage at ${cpuPct}% of capacity — throttling likely` })
  }

  for (const p of pods) {
    for (const c of p.containers || []) {
      if (c.lastReason === 'OOMKilled') {
        findings.push({ level: 'crit', text: `${p.name}/${c.name} was OOMKilled — raise memory limit or reduce usage` })
      }
    }
  }

  for (const n of snap.nodes || []) {
    for (const f of nodePressureFlags(n)) {
      if (f !== 'NotReady') {
        findings.push({ level: 'warn', text: `Node ${n.name}: ${f}` })
      } else {
        findings.push({ level: 'crit', text: `Node ${n.name}: NotReady` })
      }
    }
  }

  if (!m.available) {
    findings.push({
      level: 'info',
      text: 'Metrics-server unavailable — showing requests/limits from pod specs only',
    })
  }

  if (!findings.length) {
    findings.push({ level: 'ok', text: 'No resource pressure signals in the current window' })
  }

  return findings
}
