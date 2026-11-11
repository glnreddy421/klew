import { describe, expect, it } from 'vitest'
import { shouldPromptUpdate, updatePromptCopy } from './appUpdate.js'

describe('shouldPromptUpdate', () => {
  it('prompts only when a newer undismissed release exists', () => {
    expect(shouldPromptUpdate({ updateAvailable: true, latestVersion: '2.1.0' }, '')).toBe(true)
    expect(shouldPromptUpdate({ updateAvailable: true, latestVersion: '2.1.0' }, '2.1.0')).toBe(false)
    expect(shouldPromptUpdate({ updateAvailable: false, latestVersion: '2.1.0' }, '')).toBe(false)
    expect(shouldPromptUpdate(null, '')).toBe(false)
  })
})

describe('updatePromptCopy', () => {
  it('mentions Homebrew for cask installs', () => {
    const copy = updatePromptCopy({
      channel: 'homebrew',
      latestVersion: '2.1.0',
      currentVersion: '2.0.0',
    })
    expect(copy.title).toContain('2.1.0')
    expect(copy.message).toMatch(/Homebrew/)
  })
})
