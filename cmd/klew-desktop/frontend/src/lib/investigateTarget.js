/**
 * Investigate mode: what to investigate inside one namespace. The engine
 * lists targets grouped by kind (InvestigationTargets) from its collectors,
 * including user extensions, so the pickers show whatever it can investigate.
 */

import { fuzzyMatchFields } from './fuzzy.js'

export const WHOLE_NAMESPACE = ''
export const ALL_KINDS = ''

function normalizeGroups(groups) {
  return (Array.isArray(groups) ? groups : [])
    .filter((g) => g?.kind && Array.isArray(g.names) && g.names.length)
}

/** Kinds with object counts, in the engine's order (workloads first). */
export function kindsFromGroups(groups) {
  return normalizeGroups(groups).map((g) => ({ kind: g.kind, count: g.names.length, custom: Boolean(g.custom) }))
}

/** Objects of kind (every kind when kind is ALL_KINDS). */
export function objectsOfKind(groups, kind) {
  const out = []
  for (const g of normalizeGroups(groups)) {
    if (kind !== ALL_KINDS && g.kind !== kind) continue
    for (const name of g.names) out.push({ key: `${g.kind}/${name}`, kind: g.kind, name })
  }
  return out
}

/**
 * Fuzzy-searches every listed object by name and kind ("bil", "svc gw",
 * "deploy/led"), best match first. Each result carries the matched character
 * positions in nameHits and kindHits for highlighting.
 */
export function searchTargets(groups, query) {
  const out = []
  for (const o of objectsOfKind(groups, ALL_KINDS)) {
    const m = fuzzyMatchFields(query, { name: o.name, kind: o.kind })
    if (m) out.push({ ...o, score: m.score, nameHits: m.hits.name, kindHits: m.hits.kind })
  }
  return out.sort((a, b) => b.score - a.score || a.name.length - b.name.length || a.key.localeCompare(b.key))
}

/**
 * Query for the chosen targets: none investigates the whole namespace; one or
 * more narrow the investigation to exactly those resources, comma-separated
 * (e.g. "Deployment/api,Service/web"). Keys no longer listed are dropped.
 */
export function queryForTargets(keys, groups) {
  return reconcileTargets(keys, groups).join(',')
}

/** Keeps only targets still listed in the namespace. */
export function reconcileTargets(keys, groups) {
  const listed = new Set(objectsOfKind(groups, ALL_KINDS).map((o) => o.key))
  return (Array.isArray(keys) ? keys : []).filter((k) => listed.has(k))
}

/** Short label for the target picker button: "billing", "billing +2". */
export function targetsLabel(keys) {
  const list = Array.isArray(keys) ? keys : []
  if (!list.length) return 'All resources'
  const first = String(list[0]).split('/').pop()
  return list.length === 1 ? first : `${first} +${list.length - 1}`
}

/** Human label for what an investigation covers. */
export function targetLabel(namespace, keys) {
  if (!namespace) return ''
  const list = Array.isArray(keys) ? keys : (keys ? [keys] : [])
  if (!list.length) return `namespace ${namespace}`
  return `${list.length === 1 ? list[0] : `${list.length} resources`} in ${namespace}`
}

/** Readable plural heading for a kind: "PersistentVolumeClaim" → "Persistent volume claims". */
export function kindHeading(kind) {
  const words = String(kind || '').replace(/([a-z0-9])([A-Z])/g, '$1 $2').split(' ').filter(Boolean)
  if (!words.length) return ''
  const last = words[words.length - 1]
  words[words.length - 1] = /(s|x|ch|sh)$/i.test(last) ? `${last}es`
    : /[^aeiou]y$/i.test(last) ? `${last.slice(0, -1)}ies`
      : `${last}s`
  return words.map((w, i) => (i === 0 ? w : w.toLowerCase())).join(' ')
}
