import { describe, expect, it } from 'vitest'
import {
  isFavoriteContext,
  normalizeFavoriteContexts,
  orderContextsForDisplay,
  toggleFavoriteContext,
} from './clusterContexts.js'

describe('clusterContexts', () => {
  it('normalizes and dedupes favorites', () => {
    expect(normalizeFavoriteContexts(['a', 'a', '', ' b ', 'b'])).toEqual(['a', 'b'])
  })

  it('toggles favorites without affecting other entries', () => {
    let favs = ['prod']
    favs = toggleFavoriteContext(favs, 'staging')
    expect(favs).toEqual(['prod', 'staging'])
    favs = toggleFavoriteContext(favs, 'prod')
    expect(favs).toEqual(['staging'])
  })

  it('orders favorites first then alphabetical', () => {
    const contexts = [
      { name: 'zebra' },
      { name: 'prod' },
      { name: 'dev' },
      { name: 'staging' },
    ]
    const ordered = orderContextsForDisplay(contexts, {
      favoriteContexts: ['staging', 'prod'],
    })
    expect(ordered.map((c) => c.name)).toEqual(['staging', 'prod', 'dev', 'zebra'])
  })

  it('isFavoriteContext', () => {
    expect(isFavoriteContext(['a'], 'a')).toBe(true)
    expect(isFavoriteContext(['a'], 'b')).toBe(false)
  })
})
