import { useState } from 'react'
import { BrowserOpenURL, ClipboardSetText } from '../../../wailsjs/runtime/runtime'
import { updatePromptCopy } from '../../lib/appUpdate.js'

export function UpdateAvailableBanner({ result, onDismiss }) {
  const [copied, setCopied] = useState(false)
  if (!result?.updateAvailable) return null

  const copy = updatePromptCopy(result)
  const brew = result.channel === 'homebrew' && result.upgradeCommand

  async function copyBrew() {
    if (!result.upgradeCommand) return
    try {
      if (ClipboardSetText) {
        await ClipboardSetText(result.upgradeCommand)
      } else if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(result.upgradeCommand)
      }
      setCopied(true)
      setTimeout(() => setCopied(false), 1400)
    } catch {
      /* ignore */
    }
  }

  return (
    <div
      className="cluster-connection-banner update-available-banner tone-info"
      role="status"
      aria-live="polite"
    >
      <div className="cluster-connection-banner-main">
        <div className="cluster-connection-banner-copy">
          <strong className="cluster-connection-banner-title">{copy.title}</strong>
          <span className="cluster-connection-banner-message">{copy.message}</span>
        </div>
      </div>
      <div className="cluster-connection-banner-actions">
        {brew && (
          <button type="button" className="btn btn-outline cluster-connection-retry-btn" onClick={copyBrew}>
            {copied ? 'Copied' : 'Copy brew upgrade'}
          </button>
        )}
        {!brew && result.downloadURL && (
          <button
            type="button"
            className="btn btn-outline cluster-connection-retry-btn"
            onClick={() => BrowserOpenURL(result.downloadURL)}
          >
            Download
          </button>
        )}
        <button
          type="button"
          className="text-link-btn"
          onClick={() => BrowserOpenURL(result.releaseURL || 'https://github.com/glnreddy421/klew/releases')}
        >
          Release notes
        </button>
        {onDismiss && (
          <button
            type="button"
            className="text-link-btn cluster-connection-dismiss-btn"
            onClick={() => onDismiss(result.latestVersion)}
          >
            Not now
          </button>
        )}
      </div>
    </div>
  )
}
