/** Local font cache — IndexedDB with 1 hour TTL. */

import { idbGet, idbPut } from './localDb.js'

export const FONT_CACHE_TTL_MS = 60 * 60 * 1000
const DB_NAME = 'klew-font-cache'
const DB_VERSION = 1
const STORE = 'presets'
const META_KEY = 'klew.font-cache.meta'

/** @typedef {{ id: string, css: string, updatedAt: number }} FontCacheEntry */

export function isFontCacheStale(updatedAt) {
  if (!updatedAt) return true
  return Date.now() - updatedAt >= FONT_CACHE_TTL_MS
}

/** @returns {Promise<FontCacheEntry | null>} */
export async function readFontCache(id) {
  try {
    const entry = await idbGet(DB_NAME, DB_VERSION, STORE, id)
    if (!entry?.css) return null
    return entry
  } catch {
    return readFontCacheMetaFallback(id)
  }
}

/** @returns {Promise<void>} */
export async function writeFontCache(id, css) {
  const entry = { id, css, updatedAt: Date.now() }
  try {
    await idbPut(DB_NAME, DB_VERSION, STORE, entry)
    writeFontCacheMeta(id, entry.updatedAt)
  } catch {
    writeFontCacheMetaFallback(id, css, entry.updatedAt)
  }
}

function readFontCacheMetaFallback(id) {
  try {
    const raw = localStorage.getItem(`${META_KEY}.${id}`)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    if (!parsed?.css) return null
    return parsed
  } catch {
    return null
  }
}

function writeFontCacheMetaFallback(id, css, updatedAt) {
  try {
    localStorage.setItem(`${META_KEY}.${id}`, JSON.stringify({ id, css, updatedAt }))
  } catch {
    // quota — ignore
  }
}

function writeFontCacheMeta(id, updatedAt) {
  try {
    localStorage.setItem(META_KEY, JSON.stringify({ lastId: id, updatedAt }))
  } catch {
    // ignore
  }
}

function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer)
  const chunk = 0x8000
  let binary = ''
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(binary)
}

function mimeForFontUrl(url) {
  if (url.includes('.woff2')) return 'font/woff2'
  if (url.includes('.woff')) return 'font/woff'
  if (url.includes('.ttf')) return 'font/ttf'
  return 'font/woff2'
}

/** Fetch Google Fonts CSS and inline binary files as data URLs for offline cache. */
export async function downloadFontCss(cssUrl) {
  const res = await fetch(cssUrl, { cache: 'no-cache' })
  if (!res.ok) throw new Error(`font css ${res.status}`)
  let css = await res.text()
  const matches = [...css.matchAll(/url\((https:\/\/[^)]+)\)/g)]
  const urls = [...new Set(matches.map((m) => m[1]))]
  for (const fontUrl of urls) {
    const fontRes = await fetch(fontUrl, { cache: 'no-cache' })
    if (!fontRes.ok) continue
    const buf = await fontRes.arrayBuffer()
    const dataUrl = `data:${mimeForFontUrl(fontUrl)};base64,${arrayBufferToBase64(buf)}`
    css = css.split(fontUrl).join(dataUrl)
  }
  return css
}
