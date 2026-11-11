import { buildInspectKey } from './matches.js'

function toMillis(value) {
  const t = value ? new Date(value).getTime() : NaN
  return Number.isFinite(t) ? t : 0
}

/**
 * Group warning rows (one per object and reason, newest first) by reason so
 * eleven FailedScheduling pods read as one line with eleven objects.
 */
export function groupWarnings(events = [], { maxGroups = 6, maxObjects = 3 } = {}) {
  const byReason = new Map()
  for (const e of events) {
    if (!e) continue
    const reason = e.reason || 'Warning'
    const obj = e.involvedObject || {}
    const lastSeen = e.lastSeen || e.timestamp || ''
    let group = byReason.get(reason)
    if (!group) {
      group = { reason, count: 0, lastSeen, lastMs: 0, message: '', objects: [] }
      byReason.set(reason, group)
    }
    group.count += e.count || 1
    const ms = toMillis(lastSeen)
    if (ms >= group.lastMs) {
      group.lastMs = ms
      group.lastSeen = lastSeen
      group.message = e.message || ''
    }
    if (obj.kind && obj.name) {
      group.objects.push({
        key: buildInspectKey(obj.kind, obj.name, obj.namespace || ''),
        kind: obj.kind,
        name: obj.name,
        namespace: obj.namespace || '',
        lastMs: ms,
      })
    }
  }
  return [...byReason.values()]
    .sort((a, b) => b.lastMs - a.lastMs)
    .slice(0, maxGroups)
    .map(({ lastMs, objects, ...group }) => {
      const sorted = objects.sort((a, b) => b.lastMs - a.lastMs)
      return {
        ...group,
        objectCount: sorted.length,
        objects: sorted.slice(0, maxObjects).map(({ lastMs: _, ...o }) => o),
        moreObjects: Math.max(0, sorted.length - maxObjects),
      }
    })
}
