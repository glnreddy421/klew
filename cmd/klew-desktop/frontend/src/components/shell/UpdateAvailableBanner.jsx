import { useEffect, useState } from 'react'
import { BrowserOpenURL, ClipboardSetText } from '../../../wailsjs/runtime/runtime'
import { PrepareUpdate, RestartToUpdate } from '../../../wailsjs/go/main/App'
import { updatePromptCopy } from '../../lib/appUpdate.js'

const RELEASES_URL = 'https://github.com/glnreddy421/klew/releases'

export function UpdateAvailableBanner({ result, onDismiss }) {
  const [copied, setCopied] = useState(false)
  // Self-update: 'downloading' → 'ready' | 'failed'; 'restarting' after the click.
  const [phase, setPhase] = useState('downloading')
  const [error, setError] = useState('')
  const selfUpdate = Boolean(result?.updateAvailable && result?.canSelfUpdate && PrepareUpdate)

  useEffect(() => {
    if (!selfUpdate) return undefined
    let cancelled = false
    setPhase('downloading')
    setError('')
    PrepareUpdate()
      .then((st) => {
        if (cancelled) return
        if (st?.ready) {
          setPhase('ready')
        } else {
          setPhase('failed')
          setError(st?.error || st?.message || '')
        }
      })
      .catch((err) => {
        if (cancelled) return
        setPhase('failed')
        setError(err?.message || String(err))
      })
    return () => {
      cancelled = true
    }
  }, [selfUpdate, result?.latestVersion])

  if (!result?.updateAvailable) return null

  const copy = updatePromptCopy(result, selfUpdate ? phase : '', error)
  const brew = !selfUpdate && result.channel === 'homebrew' && result.upgradeCommand
  const showDownload = result.downloadURL && (!selfUpdate ? !brew : phase === 'failed')

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

  async function restart() {
    setPhase('restarting')
    try {
      const msg = await RestartToUpdate()
      if (msg) {
        setPhase('failed')
        setError(msg)
      }
    } catch (err) {
      setPhase('failed')
      setError(err?.message || String(err))
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
        {selfUpdate && (phase === 'ready' || phase === 'restarting') && (
          <button
            type="button"
            className="btn btn-primary cluster-connection-retry-btn"
            disabled={phase === 'restarting'}
            onClick={restart}
          >
            {phase === 'restarting' ? 'Restarting…' : 'Restart to update'}
          </button>
        )}
        {brew && (
          <button type="button" className="btn btn-outline cluster-connection-retry-btn" onClick={copyBrew}>
            {copied ? 'Copied' : 'Copy brew upgrade'}
          </button>
        )}
        {showDownload && (
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
          onClick={() => BrowserOpenURL(result.releaseURL || RELEASES_URL)}
        >
          Release notes
        </button>
        {onDismiss && phase !== 'restarting' && (
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
