/** Helpers for the desktop update prompt (check result + dismiss state). */

export function shouldPromptUpdate(result, dismissedVersion = '') {
  if (!result?.updateAvailable) return false
  const latest = String(result.latestVersion || '').trim()
  if (!latest) return false
  return latest !== String(dismissedVersion || '').trim()
}

/**
 * Banner copy. phase applies when Klew can update itself:
 * 'downloading' | 'ready' | 'failed'.
 */
export function updatePromptCopy(result, phase = '', error = '') {
  const latest = result?.latestVersion ? `v${result.latestVersion}` : 'a new version'
  const current = result?.currentVersion ? `v${result.currentVersion}` : 'this build'
  if (result?.canSelfUpdate) {
    if (phase === 'ready') {
      return {
        title: `Klew ${latest} is ready`,
        message: `Restart Klew to install it. You're on ${current}.`,
      }
    }
    if (phase === 'failed') {
      return {
        title: `Klew ${latest} is available`,
        message: `The update could not be downloaded${error ? ` (${error})` : ''}. Download it from the release page instead.`,
      }
    }
    return {
      title: `Klew ${latest} is available`,
      message: 'Downloading and verifying the update…',
    }
  }
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
