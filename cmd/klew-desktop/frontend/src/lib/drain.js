/**
 * Drain-style log template clustering (He et al.).
 * No LLM / neural model — prefix tree + token similarity.
 *
 * Pipeline: light mask → tokenize → length tree → prefix path →
 * best cluster by similarity → merge template with ***.
 */

export const DRAIN_WILDCARD = '***'

/**
 * @param {object} [opts]
 * @param {number} [opts.depth=4] search tree depth (incl. length layer)
 * @param {number} [opts.similarityThreshold=0.5]
 * @param {number} [opts.maxChildren=80]
 */
export function createDrain(opts = {}) {
  const depth = Math.max(3, opts.depth ?? 4)
  const simTh = opts.similarityThreshold ?? 0.5
  const maxChildren = opts.maxChildren ?? 80
  /** @type {Map<number, object>} length → tree node */
  const root = new Map()
  const clusters = []

  /**
   * @param {string[]} tokens
   * @param {object} meta
   */
  function add(tokens, meta = {}) {
    const seq = Array.isArray(tokens) ? tokens.filter((t) => t != null && t !== '') : []
    if (seq.length < 1) return null

    const len = seq.length
    let node = root.get(len)
    if (!node) {
      node = { children: new Map(), clusters: [] }
      root.set(len, node)
    }

    // Walk prefix layers (tokens[0..depth-3]); length already consumed one layer.
    const maxLayer = depth - 2
    for (let i = 0; i < maxLayer && i < seq.length; i++) {
      const key = tokenKey(seq[i])
      let child = node.children.get(key)
      if (!child) {
        if (node.children.size >= maxChildren) {
          // Fall back to wildcard child when fan-out is high
          child = node.children.get(DRAIN_WILDCARD)
          if (!child) {
            child = { children: new Map(), clusters: [] }
            node.children.set(DRAIN_WILDCARD, child)
          }
        } else {
          child = { children: new Map(), clusters: [] }
          node.children.set(key, child)
        }
      }
      node = child
    }

    let best = null
    let bestSim = -1
    for (const c of node.clusters) {
      const sim = clusterSimilarity(c.template, seq)
      if (sim > bestSim) {
        bestSim = sim
        best = c
      }
    }

    if (best && bestSim >= simTh) {
      best.template = mergeTemplate(best.template, seq)
      best.count += meta.weight > 0 ? meta.weight : 1
      absorbMeta(best, meta)
      return best
    }

    const cluster = {
      id: `drain_${clusters.length}_${len}`,
      template: [...seq],
      count: meta.weight > 0 ? meta.weight : 1,
      severity: meta.severity || 'info',
      pods: new Set(),
      samples: [],
      times: [],
    }
    absorbMeta(cluster, meta)
    node.clusters.push(cluster)
    clusters.push(cluster)
    return cluster
  }

  function getClusters() {
    return clusters.map((c) => ({
      ...c,
      templateStr: formatTemplate(c.template),
      pods: c.pods,
    }))
  }

  return { add, getClusters, root }
}

/**
 * Light masking + whitespace tokenization for Drain.
 * Masks obvious variables first so Drain merges on structure.
 */
export function tokenizeForDrain(message) {
  const masked = maskVariables(String(message || ''))
  if (!masked) return []
  return masked
    .split(/[\s,;=()[\]{}<>|]+/)
    .map((t) => t.replace(/^[:'"`.]+|[:'"`.]+$/g, ''))
    .filter((t) => t.length > 0)
    .slice(0, 80)
}

/** Mask high-variance tokens before Drain (preprocessor, not the clusterer). */
export function maskVariables(message) {
  let s = String(message || '')
  s = s.replace(/^\s*\[?(DEBUG|INFO|WARN(?:ING)?|ERROR|FATAL|TRACE)\]?\s*[:\-]?\s*/i, '')
  s = s.replace(
    /\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/gi,
    DRAIN_WILDCARD,
  )
  s = s.replace(/\b[0-9a-f]{16,}\b/gi, DRAIN_WILDCARD)
  s = s.replace(/\b\d{1,3}(?:\.\d{1,3}){3}(?::\d{1,5})?\b/g, DRAIN_WILDCARD)
  s = s.replace(/\b\d+ms\b/gi, DRAIN_WILDCARD)
  s = s.replace(/\b\d+µs\b/gi, DRAIN_WILDCARD)
  s = s.replace(/\b\d+\.\d+\b/g, DRAIN_WILDCARD)
  s = s.replace(/\b\d{2,}\b/g, DRAIN_WILDCARD)
  s = s.replace(/"[^"]*"/g, DRAIN_WILDCARD)
  s = s.replace(/'[^']*'/g, DRAIN_WILDCARD)
  s = s.replace(/\s+/g, ' ').trim()
  return s.slice(0, 400)
}

/**
 * Optional post-pass: merge Drain clusters with high Jaccard overlap
 * even when token lengths differ slightly (extra words).
 */
export function mergeSimilarClusters(clusters, opts = {}) {
  const thr = opts.jaccardThreshold ?? 0.72
  const list = [...(clusters || [])].sort((a, b) => b.count - a.count)
  const used = new Set()
  const out = []

  for (let i = 0; i < list.length; i++) {
    if (used.has(i)) continue
    const base = cloneCluster(list[i])
    used.add(i)
    for (let j = i + 1; j < list.length; j++) {
      if (used.has(j)) continue
      const other = list[j]
      // Only merge when lengths are close
      if (Math.abs(base.template.length - other.template.length) > 3) continue
      const jac = jaccard(contentTokens(base.template), contentTokens(other.template))
      if (jac < thr) continue
      base.template = mergeUnequalTemplates(base.template, other.template)
      base.count += other.count
      for (const p of other.pods || []) base.pods.add(p)
      base.times.push(...(other.times || []))
      for (const s of other.samples || []) {
        if (base.samples.length < 5) base.samples.push(s)
      }
      if ((SEV_RANK[other.severity] || 0) > (SEV_RANK[base.severity] || 0)) {
        base.severity = other.severity
      }
      used.add(j)
    }
    out.push(base)
  }
  return out
}

export function formatTemplate(tokens) {
  return (tokens || [])
    .join(' ')
    .replace(/(\*{3}\s*){2,}/g, `${DRAIN_WILDCARD} `)
    .trim()
}

function tokenKey(t) {
  if (t === DRAIN_WILDCARD) return DRAIN_WILDCARD
  if (/^\d+$/.test(t)) return DRAIN_WILDCARD
  return t
}

function clusterSimilarity(template, tokens) {
  if (!template?.length || template.length !== tokens.length) return 0
  let eq = 0
  for (let i = 0; i < template.length; i++) {
    const a = template[i]
    const b = tokens[i]
    if (a === DRAIN_WILDCARD || b === DRAIN_WILDCARD) continue
    if (a === b) eq += 1
  }
  return eq / template.length
}

function mergeTemplate(template, tokens) {
  const out = new Array(template.length)
  for (let i = 0; i < template.length; i++) {
    out[i] = template[i] === tokens[i] ? template[i] : DRAIN_WILDCARD
  }
  return out
}

function mergeUnequalTemplates(a, b) {
  // Align by keeping shared content tokens in order; gaps → ***
  const aa = [...a]
  const bb = [...b]
  if (aa.length === bb.length) return mergeTemplate(aa, bb)
  const longer = aa.length >= bb.length ? aa : bb
  const shorter = aa.length >= bb.length ? bb : aa
  const shortSet = new Set(contentTokens(shorter))
  return longer.map((t) => {
    if (t === DRAIN_WILDCARD) return DRAIN_WILDCARD
    if (shortSet.has(t)) return t
    return DRAIN_WILDCARD
  })
}

function contentTokens(tokens) {
  return (tokens || []).filter((t) => t && t !== DRAIN_WILDCARD)
}

function jaccard(a, b) {
  const A = new Set(a)
  const B = new Set(b)
  if (!A.size && !B.size) return 1
  let inter = 0
  for (const x of A) if (B.has(x)) inter += 1
  const union = A.size + B.size - inter
  return union ? inter / union : 0
}

function absorbMeta(cluster, meta) {
  if (meta.pod) cluster.pods.add(meta.pod)
  if (meta.ts) cluster.times.push(meta.ts)
  if (meta.sample && cluster.samples.length < 5) {
    cluster.samples.push(meta.sample)
  }
  if (meta.severity && (SEV_RANK[meta.severity] || 0) > (SEV_RANK[cluster.severity] || 0)) {
    cluster.severity = meta.severity
  }
}

function cloneCluster(c) {
  return {
    id: c.id,
    template: [...c.template],
    count: c.count,
    severity: c.severity,
    pods: new Set(c.pods || []),
    samples: [...(c.samples || [])],
    times: [...(c.times || [])],
  }
}

const SEV_RANK = {
  critical: 5,
  fatal: 5,
  high: 4,
  error: 4,
  warning: 2,
  warn: 2,
  info: 1,
  debug: 0.5,
  trace: 0.3,
}
