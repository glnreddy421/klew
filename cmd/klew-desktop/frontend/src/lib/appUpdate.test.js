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

describe('updatePromptCopy with self-update', () => {
  const result = { canSelfUpdate: true, latestVersion: '2.2.0', currentVersion: '2.1.0' }

  it('walks through downloading, ready and failed', () => {
    expect(updatePromptCopy(result, 'downloading').message).toMatch(/Downloading/)
    const ready = updatePromptCopy(result, 'ready')
    expect(ready.title).toBe('Klew v2.2.0 is ready')
    expect(ready.message).toMatch(/Restart Klew/)
    expect(updatePromptCopy(result, 'failed', 'checksum mismatch').message).toMatch(/checksum mismatch/)
  })

  it('keeps the Homebrew and download copy when Klew cannot update itself', () => {
    const copy = updatePromptCopy({ ...result, canSelfUpdate: false })
    expect(copy.message).toMatch(/Open the release/)
  })
})
