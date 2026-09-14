/** Top bar UI scale — persisted via preferences (percent). */

export const TOPBAR_SCALE_DEFAULT = 100
export const TOPBAR_SCALE_MIN = 85
export const TOPBAR_SCALE_MAX = 130
export const TOPBAR_SCALE_STEP = 5

export function normalizeTopbarScale(value) {
  const n = Number(value)
  if (!Number.isFinite(n)) return TOPBAR_SCALE_DEFAULT
  const clamped = Math.min(TOPBAR_SCALE_MAX, Math.max(TOPBAR_SCALE_MIN, Math.round(n)))
  const stepped = Math.round(clamped / TOPBAR_SCALE_STEP) * TOPBAR_SCALE_STEP
  return Math.min(TOPBAR_SCALE_MAX, Math.max(TOPBAR_SCALE_MIN, stepped))
}

export function topbarScaleFactor(percent) {
  return normalizeTopbarScale(percent) / 100
}

/** Apply CSS scale to the top bar and layout height token. */
export function applyTopbarScale(percent) {
  const normalized = normalizeTopbarScale(percent)
  const factor = normalized / 100
  const root = document.documentElement
  root.style.setProperty('--topbar-scale', String(factor))
  root.dataset.topbarScale = String(normalized)
  return normalized
}

export function bumpTopbarScale(current, delta) {
  return normalizeTopbarScale(normalizeTopbarScale(current) + delta)
}
