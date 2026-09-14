import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  CACHE_TTL,
  buildCatalogCacheKey,
  invalidateCatalogCache,
  isCatalogCacheFresh,
  loadCatalogCached,
  peekCatalogCache,
  writeCatalogCache,
} from './catalogCache.js'

afterEach(() => {
  invalidateCatalogCache()
  vi.restoreAllMocks()
})

describe('catalogCache', () => {
  it('returns fresh cache without calling fetch', async () => {
    const key = buildCatalogCacheKey('test', 'ctx')
    writeCatalogCache(key, { ok: true })
    const fetchFn = vi.fn(async () => ({ ok: false }))

    const result = await loadCatalogCached(key, CACHE_TTL.catalog, fetchFn)

    expect(result.data).toEqual({ ok: true })
    expect(result.fromCache).toBe(true)
    expect(result.revalidated).toBe(false)
    expect(fetchFn).not.toHaveBeenCalled()
  })

  it('returns stale data immediately and revalidates in background', async () => {
    vi.useFakeTimers()
    const key = buildCatalogCacheKey('stale', 'ctx')
    writeCatalogCache(key, { v: 1 })
    vi.setSystemTime(Date.now() + CACHE_TTL.entities + 1)

    let resolveFetch
    const fetchFn = vi.fn(() => new Promise((resolve) => {
      resolveFetch = resolve
    }))

    const result = await loadCatalogCached(key, CACHE_TTL.entities, fetchFn)
    expect(result.data).toEqual({ v: 1 })
    expect(result.fromCache).toBe(true)
    expect(result.revalidated).toBe(true)
    expect(fetchFn).toHaveBeenCalledTimes(1)

    resolveFetch({ v: 2 })
    await Promise.resolve()
    expect(peekCatalogCache(key)).toEqual({ v: 2 })
    vi.useRealTimers()
  })

  it('tracks freshness by ttl', () => {
    const key = buildCatalogCacheKey('freshness')
    writeCatalogCache(key, [])
    expect(isCatalogCacheFresh(key, CACHE_TTL.overview)).toBe(true)
  })
})
