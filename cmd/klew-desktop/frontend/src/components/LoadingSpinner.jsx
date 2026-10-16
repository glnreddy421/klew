import { useEffect, useState } from 'react'

/**
 * Shared loading indicators for async UI (manifests, lists, inspector, etc.).
 */

export function LoadingSpinner({ size = 'md', className = '' }) {
  return (
    <span
      className={['klew-spinner', `klew-spinner-${size}`, className].filter(Boolean).join(' ')}
      aria-hidden="true"
    />
  )
}

/** Centered block with spinner + message. */
export function LoadingState({
  message = 'Loading…',
  compact = false,
  className = '',
}) {
  return (
    <div
      className={[
        'klew-loading-state',
        compact ? 'is-compact' : '',
        className,
      ].filter(Boolean).join(' ')}
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <LoadingSpinner size={compact ? 'sm' : 'md'} />
      {message ? <span className="klew-loading-state-message">{message}</span> : null}
    </div>
  )
}

/** Inline spinner + label for toolbars, headers, and table rows. */
export function InlineLoading({ message, className = '' }) {
  return (
    <span
      className={['klew-loading-inline', className].filter(Boolean).join(' ')}
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <LoadingSpinner size="sm" />
      {message ? <span>{message}</span> : null}
    </span>
  )
}

/** Slim indeterminate bar for in-page loads (resource lists, inspector, etc.). */
export function LoadingBar({ className = '', label = 'Loading' }) {
  return (
    <div
      className={['klew-loading-bar', className].filter(Boolean).join(' ')}
      role="progressbar"
      aria-label={label}
      aria-busy="true"
      aria-valuemin={0}
      aria-valuemax={100}
    />
  )
}

/** Placeholder rows while entity lists are fetching. */
export function EntityRowsSkeleton({ rows = 7, className = '' }) {
  return (
    <div className={['klew-rows-skeleton', className].filter(Boolean).join(' ')} aria-hidden="true">
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} className="klew-skeleton-row" style={{ opacity: 1 - i * 0.06 }} />
      ))}
    </div>
  )
}

export const SLOW_LOAD_AFTER_MS = 6000

/**
 * Centered load state in the resource list/table pane. If loading runs past
 * `slowAfterMs`, reveals `slowHint` and `children` (e.g. reconnect actions).
 */
export function ResourcePaneLoading({
  label = 'Loading resources',
  className = '',
  slowHint = 'Still loading — large clusters can take a minute on first load.',
  slowAfterMs = SLOW_LOAD_AFTER_MS,
  children,
}) {
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    const id = setTimeout(() => setSlow(true), slowAfterMs)
    return () => clearTimeout(id)
  }, [slowAfterMs])

  return (
    <div
      className={['resource-pane-loading', className].filter(Boolean).join(' ')}
      role="status"
      aria-live="polite"
      aria-busy="true"
    >
      <div className="resource-pane-loading-inner">
        <LoadingBar className="resource-pane-loading-bar" label={label} />
        <EntityRowsSkeleton rows={6} className="resource-pane-loading-skeleton" />
        {slow && (slowHint || children) ? (
          <div className="resource-pane-loading-slow">
            {slowHint ? <p className="resource-pane-loading-hint muted">{slowHint}</p> : null}
            {children}
          </div>
        ) : null}
      </div>
    </div>
  )
}
