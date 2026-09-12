/** Line count above which syntax-highlight rendering is skipped. */
export const YAML_HIGHLIGHT_MAX_LINES = 400

const MAX_CACHE_ENTRIES = 48

/** @type {Map<string, { lineCount: number, lines: string[] }>} */
const lineCache = new Map()

/** Stable session cache key from manifest YAML content. */
export function yamlContentKey(text) {
  const value = String(text ?? '')
  if (!value) return ''
  let hash = 2166136261
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return `${value.length}:${hash >>> 0}`
}

function touchCache(key, entry) {
  if (lineCache.has(key)) lineCache.delete(key)
  lineCache.set(key, entry)
  while (lineCache.size > MAX_CACHE_ENTRIES) {
    const oldest = lineCache.keys().next().value
    lineCache.delete(oldest)
  }
}

/** Split YAML into lines with a small LRU cache (safe across remounts). */
export function getYamlLines(text) {
  const value = String(text ?? '')
  const key = yamlContentKey(value)
  if (!key) return []

  const cached = lineCache.get(key)
  if (cached) {
    touchCache(key, cached)
    return cached.lines
  }

  const lines = value.length ? value.split('\n') : []
  touchCache(key, { lineCount: lines.length, lines })
  return lines
}

export function getYamlLineCount(text) {
  return getYamlLines(text).length
}

export function shouldHighlightYaml(lineCount) {
  return lineCount > 0 && lineCount <= YAML_HIGHLIGHT_MAX_LINES
}

/**
 * @returns {{ mode: 'plain' | 'highlight', lineCount: number, lines: string[], cacheKey: string }}
 */
export function getYamlRenderMeta(text) {
  const value = String(text ?? '')
  const cacheKey = yamlContentKey(value)
  if (!cacheKey) {
    return { mode: 'plain', lineCount: 0, lines: [], cacheKey: '' }
  }
  const lines = getYamlLines(value)
  const lineCount = lines.length
  return {
    mode: shouldHighlightYaml(lineCount) ? 'highlight' : 'plain',
    lineCount,
    lines,
    cacheKey,
  }
}

export function clearYamlRenderCache() {
  lineCache.clear()
}
