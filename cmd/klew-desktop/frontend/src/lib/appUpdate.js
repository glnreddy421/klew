/** Helpers for the desktop update prompt (check result + dismiss state). */

export function shouldPromptUpdate(result, dismissedVersion = '') {
  if (!result?.updateAvailable) return false
  const latest = String(result.latestVersion || '').trim()
  if (!latest) return false
  return latest !== String(dismissedVersion || '').trim()
}

export function updatePromptCopy(result) {
  const latest = result?.latestVersion ? `v${result.latestVersion}` : 'a new version'
  const current = result?.currentVersion ? `v${result.currentVersion}` : 'this build'
  if (result?.channel === 'homebrew') {
    return {
      title: `Klew ${latest} is available`,
      message: `You're on ${current}. Upgrade with Homebrew, or open the release notes.`,
    }
  }
  return {
    title: `Klew ${latest} is available`,
    message: `You're on ${current}. Open the release to download the update.`,
  }
}
