import { describe, expect, it } from 'vitest'
import {
  addContextTag,
  collectContextTags,
  detailsForContext,
  displayContextName,
  enableRecordingUnlessOptedOut,
  filterContextsForHome,
  formatContextDetailMeta,
  isFavoriteContext,
  mergeContextDetailsFromContexts,
  nextPreferredContext,
  normalizeContextTags,
  normalizeFavoriteContexts,
  normalizeTag,
  orderContextsForDisplay,
  preferredContextNames,
  removeContextTag,
  setContextAlias,
  tagsForContext,
  toggleFavoriteContext,
  toggleRecordedContext,
  truncateContextLabel,
  upsertContextDetails,
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

  it('toggles per-context recording with opt-out memory', () => {
    let state = toggleRecordedContext(['a'], [], 'b')
    expect(state).toEqual({ recordedContexts: ['a', 'b'], recordingOptOutContexts: [] })
    state = toggleRecordedContext(state.recordedContexts, state.recordingOptOutContexts, 'a')
    expect(state).toEqual({ recordedContexts: ['b'], recordingOptOutContexts: ['a'] })
    expect(enableRecordingUnlessOptedOut(state.recordedContexts, state.recordingOptOutContexts, 'a'))
      .toEqual({ recordedContexts: ['b'], recordingOptOutContexts: ['a'] })
    expect(enableRecordingUnlessOptedOut(state.recordedContexts, state.recordingOptOutContexts, 'c'))
      .toEqual({ recordedContexts: ['b', 'c'], recordingOptOutContexts: ['a'] })
  })

  it('orders default first, then favorites, then alphabetical', () => {
    const contexts = [
      { name: 'zebra' },
      { name: 'prod' },
      { name: 'dev' },
      { name: 'staging' },
    ]
    expect(preferredContextNames(contexts, {
      defaultContext: 'dev',
      favoriteContexts: ['staging', 'prod'],
    })).toEqual(['dev', 'staging', 'prod', 'zebra'])
    expect(orderContextsForDisplay(contexts, {
      defaultContext: 'dev',
      favoriteContexts: ['staging', 'prod'],
    }).map((c) => c.name)).toEqual(['dev', 'staging', 'prod', 'zebra'])
    expect(orderContextsForDisplay(contexts, {
      favoriteContexts: ['staging', 'prod'],
    }).map((c) => c.name)).toEqual(['staging', 'prod', 'dev', 'zebra'])
  })

  it('falls back to the next preferred context', () => {
    const contexts = [
      { name: 'zebra' },
      { name: 'prod' },
      { name: 'dev' },
      { name: 'staging' },
    ]
    expect(nextPreferredContext(contexts, {
      defaultContext: 'dev',
      favoriteContexts: ['staging', 'prod'],
      current: 'dev',
    })).toBe('staging')
    expect(nextPreferredContext(contexts, {
      defaultContext: 'dev',
      favoriteContexts: ['staging', 'prod'],
      current: 'staging',
      alreadyTried: ['dev'],
    })).toBe('prod')
    expect(nextPreferredContext(contexts, {
      defaultContext: 'dev',
      favoriteContexts: ['staging', 'prod'],
      current: 'zebra',
      alreadyTried: ['dev', 'staging', 'prod'],
    })).toBe('')
  })

  it('isFavoriteContext', () => {
    expect(isFavoriteContext(['a'], 'a')).toBe(true)
    expect(isFavoriteContext(['a'], 'b')).toBe(false)
  })

  it('normalizes tags', () => {
    expect(normalizeTag('  Prod East  ')).toBe('prod-east')
    expect(normalizeTag('Lab!!!')).toBe('lab')
    expect(normalizeTag('')).toBe('')
  })

  it('adds and removes context tags', () => {
    let tags = addContextTag({}, 'docker-desktop', 'Lab')
    expect(tagsForContext(tags, 'docker-desktop')).toEqual(['lab'])
    tags = addContextTag(tags, 'docker-desktop', 'prod')
    tags = addContextTag(tags, 'docker-desktop', 'lab')
    expect(tagsForContext(tags, 'docker-desktop')).toEqual(['lab', 'prod'])
    tags = removeContextTag(tags, 'docker-desktop', 'lab')
    expect(tagsForContext(tags, 'docker-desktop')).toEqual(['prod'])
    tags = removeContextTag(tags, 'docker-desktop', 'prod')
    expect(normalizeContextTags(tags)).toEqual({})
  })

  it('aliases and truncates long context display names', () => {
    expect(truncateContextLabel('arn:aws:eks:us-east-1:123456789012:cluster/payments-prod', 28))
      .toBe('arn:aws:eks:us-east-1:12345…')
    let aliases = setContextAlias({}, 'arn:aws:eks:us-east-1:123456789012:cluster/payments-prod', 'payments')
    expect(displayContextName(aliases, 'arn:aws:eks:us-east-1:123456789012:cluster/payments-prod')).toBe('payments')
    aliases = setContextAlias(aliases, 'arn:aws:eks:us-east-1:123456789012:cluster/payments-prod', '')
    expect(displayContextName(aliases, 'arn:aws:eks:us-east-1:123456789012:cluster/payments-prod'))
      .toBe('arn:aws:eks:us-east-1:12345…')
  })

  it('stores and formats learned context details for tiles', () => {
    let details = upsertContextDetails({}, 'kind-klew-a', { version: 'v1.29.2' })
    // Cluster label that only repeats the context name is ignored when merging rows.
    details = mergeContextDetailsFromContexts(details, [
      { name: 'kind-klew-a', cluster: 'kind-klew-a', version: 'v1.29.2' },
      { name: 'prod-east', cluster: 'eks-prod', version: 'v1.28.4' },
    ])
    expect(detailsForContext(details, 'kind-klew-a')).toEqual({ version: 'v1.29.2' })
    expect(formatContextDetailMeta(detailsForContext(details, 'kind-klew-a'), 'kind-klew-a')).toBe('v1.29.2')
    expect(formatContextDetailMeta(detailsForContext(details, 'prod-east'), 'prod-east')).toBe('eks-prod · v1.28.4')
    const same = upsertContextDetails(details, 'kind-klew-a', { version: 'v1.29.2' })
    expect(same).toBe(details)
  })

  it('returns the same object when a row adds nothing (no render loop)', () => {
    const details = { 'prod-east': { cluster: 'eks-prod', version: 'v1.28.4' } }
    // Never connected, and its cluster name only repeats the context name.
    const merged = mergeContextDetailsFromContexts(details, [
      { name: 'kind-klew-b', cluster: 'kind-klew-b' },
    ])
    expect(merged).toBe(details)
    expect(upsertContextDetails(details, 'kind-klew-b', {})).toBe(details)
  })

  it('collects unique tags and filters contexts', () => {
    const contexts = [
      { name: 'prod-east', cluster: 'eks', namespace: 'payments' },
      { name: 'docker-desktop', cluster: 'docker-desktop', namespace: 'klew-lab' },
      { name: 'staging', cluster: 'gke' },
    ]
    const contextTags = {
      'prod-east': ['prod', 'payments'],
      'docker-desktop': ['lab'],
    }
    expect(collectContextTags(contextTags)).toEqual(['lab', 'payments', 'prod'])
    expect(filterContextsForHome(contexts, {
      favoriteContexts: ['staging'],
      contextTags,
      query: 'lab',
    }).map((c) => c.name)).toEqual(['docker-desktop'])
    expect(filterContextsForHome(contexts, {
      favoriteContexts: ['staging'],
      contextTags,
      tag: 'prod',
    }).map((c) => c.name)).toEqual(['prod-east'])
    expect(filterContextsForHome(contexts, {
      favoriteContexts: ['staging'],
      contextTags,
      query: 'gke',
    }).map((c) => c.name)).toEqual(['staging'])
  })
})
