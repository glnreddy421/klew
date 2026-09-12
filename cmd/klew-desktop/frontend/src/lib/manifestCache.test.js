import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  clearManifestCache,
  loadResourceManifest,
  manifestCacheKey,
  peekManifestCache,
  prefetchResourceManifest,
  storeManifestCache,
} from './manifestCache.js'

vi.mock('../../wailsjs/go/main/App', () => ({
  GetResourceManifest: vi.fn(),
}))

import { GetResourceManifest } from '../../wailsjs/go/main/App'

const cluster = { selectedContext: 'dev', kubeconfigPath: '/tmp/kube' }
const target = {
  kind: 'Pod',
  name: 'redis-abc',
  namespace: 'klew-lab',
  resourceId: 'v1/pods',
  clusterScoped: false,
}

afterEach(() => {
  clearManifestCache()
  vi.clearAllMocks()
})

describe('manifestCacheKey', () => {
  it('includes context, kubeconfig, and resource identity', () => {
    expect(manifestCacheKey(target, cluster)).toBe('/tmp/kube|dev|v1/pods|klew-lab|redis-abc')
  })
})

describe('manifest cache', () => {
  it('stores and retrieves cached manifests', () => {
    const key = manifestCacheKey(target, cluster)
    const data = { yaml: 'apiVersion: v1', command: 'kubectl get pod redis-abc -o yaml' }
    storeManifestCache(key, data)
    expect(peekManifestCache(key)).toEqual(data)
  })

  it('dedupes concurrent loads for the same key', async () => {
    GetResourceManifest.mockImplementation(() => new Promise((resolve) => {
      setTimeout(() => resolve({ yaml: 'kind: Pod', command: 'kubectl get pod' }), 20)
    }))

    const key = manifestCacheKey(target, cluster)
    const first = loadResourceManifest(target, cluster, key)
    const second = loadResourceManifest(target, cluster, key)

    expect(first).toBe(second)
    await first
    expect(GetResourceManifest).toHaveBeenCalledTimes(1)
    expect(peekManifestCache(key)?.yaml).toBe('kind: Pod')
  })

  it('skips prefetch when cache entry is still fresh', async () => {
    const key = manifestCacheKey(target, cluster)
    storeManifestCache(key, { yaml: 'cached', command: 'kubectl get pod' })

    await prefetchResourceManifest(target, cluster)
    expect(GetResourceManifest).not.toHaveBeenCalled()
  })
})
