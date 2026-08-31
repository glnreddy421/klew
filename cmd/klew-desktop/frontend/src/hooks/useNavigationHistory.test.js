/**
 * Run: node --test src/hooks/useNavigationHistory.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  HOME_NAV,
  navEntryKey,
  normalizeNavEntry,
} from './useNavigationHistory.js'

test('normalizeNavEntry merges partial navigation targets', () => {
  const entry = normalizeNavEntry(
    { tab: 'nodes', nodesMode: 'scope' },
    { tab: 'incident', nodesFocus: 'cluster', settingsSection: 'general' },
  )
  assert.equal(entry.tab, 'nodes')
  assert.equal(entry.nodesFocus, 'scope')
  assert.equal(entry.settingsSection, 'general')
})

test('navEntryKey distinguishes settings sections', () => {
  assert.notEqual(
    navEntryKey({ tab: 'settings', settingsSection: 'general' }),
    navEntryKey({ tab: 'settings', settingsSection: 'help' }),
  )
})

test('HOME_NAV points at overview', () => {
  assert.equal(HOME_NAV.tab, 'incident')
})
