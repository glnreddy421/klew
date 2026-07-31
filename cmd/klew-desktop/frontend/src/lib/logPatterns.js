/**
 * Log pattern atoms for Live tail · Patterns (Gonzo-style dashboard).
 * Templates use *** placeholders; includes top words, attributes, severity counts, histogram.
 */

import {
  normalizePodSelection,
  resolveEventPod,
  logMatchesSearch,
} from './streamView.js'
import {
  createDrain,
  tokenizeForDrain,
  mergeSimilarClusters,
  formatTemplate,
} from './drain.js'

const SOURCE_LOG = 'log'

const STOPWORDS = new Set([
  'a', 'an', 'the', 'to', 'of', 'in', 'on', 'for', 'with', 'and', 'or', 'at',
  'by', 'from', 'is', 'are', 'was', 'were', 'be', 'been', 'as', 'it', 'this',
  'that', 'not', 'no', 'yes', 'http', 'https', 'get', 'post', 'put', 'patch',
  'delete', 'null', 'undefined', 'true', 'false', 'info', 'warn', 'warning',
  'error', 'err', 'fatal', 'debug', 'trace', 'level', 'msg', 'message', 'time',
  'timestamp', 'ts', 'ms', 'ns', 'pod', 'container', 'kube', 'k8s', 'via',
  'from', 'into', 'over', 'under', 'than', 'then', 'also', 'just', 'only',
])

const SEV_WEIGHT = {
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

const HIST_BUCKETS = 24

/**
 * @returns {{
 *   templates: Array<object>,
 *   words: Array<{rank:number, word:string, count:number, score?:number}>,
 *   attributes: Array<{rank:number, key:string, count:number}>,
 *   severity: Record<string, number>,
 *   histogram: number[],
 *   window: object
 * }}
 */
export function extractLogPatterns(evidence, opts = {}) {
  const selected = normalizePodSelection(opts.pods)
  const search = opts.search || ''
  // Gonzo-style: analyze full log window by default
  const preferErrors = opts.errorOnly === true
  const maxTemplates = opts.maxTemplates || 40
  const maxWords = opts.maxWords || 10
  const maxAttrs = opts.maxAttributes || 10
  const maxSamples = opts.maxSamples || 5

  let allLogs = (Array.isArray(evidence) ? evidence : []).filter((e) => {
    const t = e.sourceType || e.SourceType
    return String(t || '').toLowerCase() === SOURCE_LOG
  })

  if (selected.length) {
    const set = new Set(selected)
    allLogs = allLogs.filter((e) => {
      const name = resolveEventPod(e)
      return name && set.has(name)
    })
  }

  if (search.trim()) {
    allLogs = allLogs.filter((e) => {
      const hay = [
        e.pod || e.Pod,
        e.container || e.Container,
        e.message || e.Message,
        e.raw || e.Raw,
        resolveEventPod(e),
      ].join(' ')
      return logMatchesSearch(hay, search)
    })
  }

  let lines = allLogs
  let scope = 'all'
  if (preferErrors) {
    const signal = allLogs.filter((e) => isSignalSeverity(e.severity || e.Severity))
    if (signal.length) {
      lines = signal
      scope = 'signal'
    }
  }

  const drain = createDrain({
    depth: opts.drainDepth ?? 4,
    similarityThreshold: opts.drainSimilarity ?? 0.5,
  })
  const attrCounts = new Map()
  const podsSeen = new Set()
  const severity = {
    fatal: 0,
    error: 0,
    warn: 0,
    info: 0,
    debug: 0,
    trace: 0,
  }
  const times = []
  /** Per-line token sets for TF–IDF (each log line = one document). */
  const docs = []

  let analyzed = 0
  for (const e of lines) {
    const raw = stripPrefix(e)
    if (!raw || raw.length < 2) continue

    const tokens = tokenizeForDrain(raw)
    if (tokens.length < 1) continue

    const count = Number(e.count || e.Count || 1) || 1
    analyzed += count

    const pod = resolveEventPod(e)
    if (pod) podsSeen.add(pod)
    const sev = String(e.severity || e.Severity || 'info').toLowerCase()
    bumpSeverity(severity, sev, count)

    const ts = toMs(e.timestamp || e.Timestamp)
    if (ts) times.push(...Array(Math.min(count, 8)).fill(ts))

    drain.add(tokens, {
      weight: count,
      pod,
      severity: sev,
      ts,
      sample: {
        message: raw,
        pod: pod || '',
        container: e.container || e.Container || '',
        timestamp: e.timestamp || e.Timestamp || null,
        severity: sev,
      },
    })

    const wordTokens = tokenizeWords(raw)
    docs.push({ tokens: wordTokens, ts: ts || 0, weight: count })

    for (const key of extractAttributes(raw)) {
      attrCounts.set(key, (attrCounts.get(key) || 0) + count)
    }
  }

  const totalForPct = Math.max(1, analyzed)
  const now = Date.now()
  const merged = mergeSimilarClusters(drain.getClusters(), {
    jaccardThreshold: opts.drainJaccard ?? 0.72,
  })

  const templates = merged
    .map((b) => {
      const tpl = b.templateStr || formatTemplate(b.template)
      const trend = computeTrend(b.times, now)
      const score = scoreTemplate(b, trend)
      const pct = (b.count / totalForPct) * 100
      return {
        id: hashId(tpl),
        template: tpl,
        count: b.count,
        pct,
        trend,
        severity: b.severity,
        pods: [...b.pods].sort(),
        samples: b.samples,
        score,
      }
    })
    .sort((a, b) => b.count - a.count || b.score - a.score)
    .slice(0, maxTemplates)

  const words = rankTopWordsTfIdf(docs, { maxWords })

  const attributes = [...attrCounts.entries()]
    .map(([key, count]) => ({ key, count }))
    .sort((a, b) => b.count - a.count || a.key.localeCompare(b.key))
    .slice(0, maxAttrs)
    .map((a, i) => ({ rank: i + 1, ...a }))

  const histogram = buildHistogram(times, HIST_BUCKETS)

  return {
    templates,
    words,
    attributes,
    severity,
    histogram,
    window: {
      lineCount: analyzed,
      totalLogs: allLogs.length,
      patternCount: templates.length,
      podCount: podsSeen.size,
      errorOnly: preferErrors,
      scope,
      histMax: histogram.length ? Math.max(...histogram, 0) : 0,
      wordModel: '',
      templateModel: '',
    },
  }
}

export function scoreTemplates(templates) {
  return [...(templates || [])].sort((a, b) => (b.score || 0) - (a.score || 0))
}

/**
 * TF–IDF + residual-IDF top-word ranker (no LLM).
 *
 * Each log line is a document. Split by time into baseline (older half) vs
 * current (newer half). Score favors terms that are frequent now, rare across
 * documents, and elevated vs the baseline half:
 *
 *   score = tf_now × idf × ln(1 + tf_now / (tf_base + 1))
 *
 * @param {Array<{ tokens: string[], ts: number, weight?: number }>} docs
 * @returns {Array<{ rank: number, word: string, count: number, score: number, tf: number, idf: number, lift: number }>}
 */
export function rankTopWordsTfIdf(docs, opts = {}) {
  const maxWords = opts.maxWords || 10
  const list = Array.isArray(docs) ? docs.filter((d) => d?.tokens?.length) : []
  if (!list.length) return []

  const sorted = [...list].sort((a, b) => (a.ts || 0) - (b.ts || 0))
  const split = Math.max(1, Math.floor(sorted.length / 2))
  const baselineDocs = sorted.length >= 4 ? sorted.slice(0, split) : []
  const currentDocs = sorted.length >= 4 ? sorted.slice(split) : sorted

  const nDocs = sorted.length
  const df = new Map()
  const tfAll = new Map()
  const tfNow = new Map()
  const tfBase = new Map()

  for (const d of sorted) {
    const w = d.weight > 0 ? d.weight : 1
    const uniq = new Set(d.tokens)
    for (const t of uniq) df.set(t, (df.get(t) || 0) + 1)
    for (const t of d.tokens) tfAll.set(t, (tfAll.get(t) || 0) + w)
  }
  for (const d of currentDocs) {
    const w = d.weight > 0 ? d.weight : 1
    for (const t of d.tokens) tfNow.set(t, (tfNow.get(t) || 0) + w)
  }
  for (const d of baselineDocs) {
    const w = d.weight > 0 ? d.weight : 1
    for (const t of d.tokens) tfBase.set(t, (tfBase.get(t) || 0) + w)
  }

  // When the window is too short for a baseline split, rank on classic TF–IDF only.
  const useResidual = baselineDocs.length > 0
  const scored = []
  for (const [word, tf] of tfNow.entries()) {
    if (tf < 1) continue
    const docFreq = df.get(word) || 1
    const idf = Math.log((nDocs + 1) / (docFreq + 1)) + 1
    const base = tfBase.get(word) || 0
    const lift = useResidual ? tf / (base + 1) : 1
    const residual = useResidual ? Math.log(1 + lift) : 1
    const score = tf * idf * residual
    scored.push({
      word,
      count: tfAll.get(word) || tf,
      tf,
      idf: round4(idf),
      lift: round4(lift),
      score: round4(score),
    })
  }

  return scored
    .sort((a, b) => b.score - a.score || b.count - a.count || a.word.localeCompare(b.word))
    .slice(0, maxWords)
    .map((w, i) => ({ rank: i + 1, ...w }))
}

function round4(n) {
  return Math.round(n * 10000) / 10000
}

/** Normalize to Gonzo-like templates with *** placeholders. */
export function normalizeTemplate(message) {
  let s = String(message || '')
  s = s.replace(/^\s*\[?(DEBUG|INFO|WARN(?:ING)?|ERROR|FATAL|TRACE)\]?\s*[:\-]?\s*/i, '')
  // Keep structured {field} names as-is (already placeholders)
  s = s.replace(
    /\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b/gi,
    '***',
  )
  s = s.replace(/\b[0-9a-f]{16,}\b/gi, '***')
  s = s.replace(/\b\d{1,3}(?:\.\d{1,3}){3}(?::\d{1,5})?\b/g, '***')
  s = s.replace(/\b\d+ms\b/gi, '***')
  s = s.replace(/\b\d+µs\b/gi, '***')
  s = s.replace(/\b\d+\.\d+\b/g, '***')
  s = s.replace(/\b\d{2,}\b/g, '***')
  s = s.replace(/"[^"]*"/g, '***')
  s = s.replace(/'[^']*'/g, '***')
  // Collapse path segments that look like ids
  s = s.replace(/\/[0-9a-z._-]{10,}/gi, '/***')
  s = s.replace(/\s+/g, ' ').trim()
  // Collapse runs of ***
  s = s.replace(/(\*{3}\s*){2,}/g, '*** ')
  s = s.replace(/(\*{3}\/){2,}/g, '***/')
  return s.slice(0, 200)
}

export function tokenizeWords(message) {
  const s = String(message || '').toLowerCase()
  const out = []
  const re = /[a-z][a-z0-9_./:-]{2,}/g
  let m
  while ((m = re.exec(s))) {
    let w = m[0]
    if (w.includes('://')) continue
    if (/^\d/.test(w)) continue
    if (STOPWORDS.has(w)) continue
    if (w.includes('/')) w = w.split('/').pop()
    if (!w || w.length < 3 || STOPWORDS.has(w)) continue
    // Keep host:port style tokens (gonzo shows them)
    out.push(w.slice(0, 48))
  }
  return out
}

/** @deprecated alias */
export function tokenizeErrorWords(message) {
  return tokenizeWords(message)
}

export function extractAttributes(message) {
  const s = String(message || '')
  const keys = []
  const re = /\b([a-zA-Z_][\w.-]{1,40})\s*[=:]\s*(?:"[^"]*"|'[^']*'|[^\s,;]+)/g
  let m
  while ((m = re.exec(s))) {
    const key = m[1]
    if (STOPWORDS.has(key.toLowerCase())) continue
    if (/^(DEBUG|INFO|WARN|ERROR|FATAL|TRACE)$/i.test(key)) continue
    keys.push(key)
  }
  return keys
}

function stripPrefix(e) {
  let m = String(e.message || e.Message || e.raw || e.Raw || '')
  const pod = resolveEventPod(e)
  const container = e.container || e.Container
  if (pod && container) m = m.replace(`${pod}/${container}: `, '')
  if (pod) m = m.replace(`${pod}: `, '')
  return m.trim()
}

function scoreTemplate(b, trend) {
  const sev = SEV_WEIGHT[b.severity] || 1
  let score = b.count * (1 + sev * 0.15)
  if (trend === '↑') score *= 1.2
  else if (trend === '↓') score *= 0.9
  score += (b.pods?.size || 0)
  return score
}

function computeTrend(times, now) {
  if (!times.length) return '·'
  const sorted = [...times].filter((t) => Number.isFinite(t)).sort((a, b) => a - b)
  if (sorted.length < 3) return '·'
  const span = sorted[sorted.length - 1] - sorted[0]
  const cutoff = span > 0 ? sorted[0] + span / 2 : now - 60_000
  const recent = sorted.filter((t) => t >= cutoff).length
  const ratio = recent / sorted.length
  if (ratio >= 0.65) return '↑'
  if (ratio <= 0.35) return '↓'
  return '·'
}

function bumpSeverity(severity, sev, count) {
  if (sev === 'critical' || sev === 'fatal') severity.fatal += count
  else if (sev === 'high' || sev === 'error') severity.error += count
  else if (sev === 'warning' || sev === 'warn') severity.warn += count
  else if (sev === 'debug') severity.debug += count
  else if (sev === 'trace') severity.trace += count
  else severity.info += count
}

function isSignalSeverity(sev) {
  const s = String(sev || '').toLowerCase()
  return s === 'critical' || s === 'high' || s === 'warning' || s === 'warn' || s === 'error' || s === 'fatal'
}

function toMs(ts) {
  if (!ts) return 0
  const n = new Date(ts).getTime()
  return Number.isFinite(n) ? n : 0
}

function buildHistogram(times, n) {
  const hist = Array(n).fill(0)
  if (!times.length) return hist
  const sorted = times.filter((t) => t > 0).sort((a, b) => a - b)
  if (!sorted.length) return hist
  const min = sorted[0]
  const max = sorted[sorted.length - 1]
  const span = Math.max(1, max - min)
  for (const t of sorted) {
    let i = Math.floor(((t - min) / span) * n)
    if (i >= n) i = n - 1
    if (i < 0) i = 0
    hist[i] += 1
  }
  return hist
}

function hashId(s) {
  let h = 0
  for (let i = 0; i < s.length; i++) h = ((h << 5) - h + s.charCodeAt(i)) | 0
  return `tpl_${(h >>> 0).toString(36)}`
}
