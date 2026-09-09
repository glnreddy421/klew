/**
 * Run: node --test src/lib/clusterConnection.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  classifyConnectionError,
  deriveConnectionState,
  isClusterDisconnected,
  connectionErrorMessage,
  connectionGuidance,
} from './clusterConnection.js'

const cluster = {
  selectedContext: 'prod-east',
  syncedAt: new Date().toISOString(),
  syncError: '',
}

test('classifyConnectionError detects AWS auth failures', () => {
  const auth = classifyConnectionError('api server: authentication failed — exec plugin aws eks get-token: token expired')
  assert.equal(auth.kind, 'auth')
  assert.match(auth.guidance, /aws sso login/i)
})

test('deriveConnectionState idle without context', () => {
  const state = deriveConnectionState({ cluster: { ...cluster, selectedContext: '' } })
  assert.equal(state.phase, 'idle')
  assert.equal(state.showBanner, false)
})

test('deriveConnectionState connecting while switching context', () => {
  const state = deriveConnectionState({ cluster, connecting: true })
  assert.equal(state.phase, 'connecting')
  assert.equal(state.showBanner, true)
  assert.match(state.message, /Switching to prod-east/)
})

test('deriveConnectionState disconnected on sync error', () => {
  const state = deriveConnectionState({
    cluster: { ...cluster, syncError: 'connection refused' },
  })
  assert.equal(state.phase, 'disconnected')
  assert.equal(state.showBanner, true)
  assert.equal(state.message, 'connection refused')
  assert.equal(state.showRetry, true)
})

test('deriveConnectionState auth disconnect highlights credentials', () => {
  const state = deriveConnectionState({
    cluster: {
      ...cluster,
      syncError: 'list namespaces: authentication failed — token expired',
    },
  })
  assert.match(state.title, /credentials/i)
  assert.match(state.detail, /AWS or EKS login/i)
})

test('deriveConnectionState retrying shows countdown', () => {
  const state = deriveConnectionState({
    cluster: { ...cluster, syncError: 'timeout' },
    retryInSec: 5,
    maxRetries: 5,
  })
  assert.equal(state.phase, 'retrying')
  assert.match(state.detail, /Retrying in 5s/)
})

test('deriveConnectionState exhausted shows reconnect attempts', () => {
  const state = deriveConnectionState({
    cluster: {
      ...cluster,
      syncError: 'authentication failed — credentials may have expired',
    },
    autoRetryExhausted: true,
    retryAttempt: 5,
    maxRetries: 5,
  })
  assert.match(state.title, /Could not connect/)
  assert.match(state.detail, /tried reconnecting 5 times/i)
  assert.match(state.detail, /Refresh AWS or EKS credentials/i)
})

test('connectionGuidance auth exhausted', () => {
  const guidance = connectionGuidance(
    { syncError: 'Unauthorized' },
    null,
    { autoRetryExhausted: true, retryAttempt: 5, maxRetries: 5 },
  )
  assert.match(guidance, /could not authenticate/i)
})

test('isClusterDisconnected prefers sync error over status loading', () => {
  assert.equal(
    isClusterDisconnected({ ...cluster, syncError: 'denied' }, null),
    true,
  )
})

test('deriveConnectionState ignores background status refresh when already connected', () => {
  const state = deriveConnectionState({
    cluster,
    statusLoading: true,
    clusterStatus: { available: true, apiReachable: true },
  })
  assert.equal(state.phase, 'connected')
  assert.equal(state.showBanner, false)
})

test('deriveConnectionState connecting on first status check', () => {
  const state = deriveConnectionState({
    cluster,
    statusLoading: true,
    clusterStatus: null,
  })
  assert.equal(state.phase, 'connecting')
  assert.equal(state.showBanner, true)
})

test('connectionErrorMessage prefers sync error', () => {
  assert.equal(
    connectionErrorMessage(
      { syncError: 'list namespaces: timeout' },
      { error: 'api server: down' },
    ),
    'list namespaces: timeout',
  )
})
